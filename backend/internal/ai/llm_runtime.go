package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"school-exam-studio/backend/internal/auth"
)

var (
	// ErrModelUnavailable 表示已配置模型但本次调用在重试与故障转移后仍失败。
	ErrModelUnavailable = errors.New("ai model unavailable")

	errAllModelsOpen = errors.New("所有模型均处于熔断状态")
)

const (
	modelMaxAttempts   = 3
	modelMaxCandidates = 3
	defaultBreakerOpen = 60 * time.Second
	breakerThreshold   = 3
)

// modelCandidate 是一个可尝试的模型连接，Key 用于熔断计数。
type modelCandidate struct {
	Client *LLMClient
	Config LLMConfig
	Key    string
}

type breakerEntry struct {
	failures    int
	openedUntil time.Time
}

// breakerSet 是进程内熔断器，按「学校|模型」维度统计连续失败。
type breakerSet struct {
	mu        sync.Mutex
	cooldown  time.Duration
	threshold int
	entries   map[string]*breakerEntry
}

func newBreakerSet(cooldown time.Duration, threshold int) *breakerSet {
	if cooldown <= 0 {
		cooldown = defaultBreakerOpen
	}
	if threshold <= 0 {
		threshold = breakerThreshold
	}
	return &breakerSet{cooldown: cooldown, threshold: threshold, entries: map[string]*breakerEntry{}}
}

// allow 在熔断窗口内返回 false；窗口结束后放行一次用于半开探测。
func (b *breakerSet) allow(key string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.entries[key]
	if !ok {
		return true
	}
	if entry.openedUntil.IsZero() {
		return true
	}
	return !now.Before(entry.openedUntil)
}

func (b *breakerSet) failure(key string, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.entries[key]
	if !ok {
		entry = &breakerEntry{}
		b.entries[key] = entry
	}
	entry.failures++
	if entry.failures >= b.threshold {
		entry.openedUntil = now.Add(b.cooldown)
	}
}

func (b *breakerSet) success(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, key)
}

func (b *breakerSet) state(key string) (int, time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.entries[key]
	if !ok {
		return 0, time.Time{}
	}
	return entry.failures, entry.openedUntil
}

func (s *Service) breaker() *breakerSet {
	if s.breakers == nil {
		s.breakers = newBreakerSet(defaultBreakerOpen, breakerThreshold)
	}
	return s.breakers
}

func (s *Service) allowLocalFallback() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("AI_ALLOW_LOCAL_FALLBACK")), "true")
}

// resolveCandidates 返回按「默认模型 → 其余启用模型」排序的候选连接，最多 3 个。
func (s *Service) resolveCandidates(ctx context.Context, principal auth.Principal, requestedModel string) ([]modelCandidate, string, error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if s.settingsRepo != nil {
		record, err := s.settingsRepo.GetSchoolAISetting(ctx, principal.SchoolID)
		if err == nil {
			if !record.Enabled {
				return nil, "", nil
			}
			if s.settingsCipher == nil {
				return nil, "服务器未配置 AI 设置加密密钥，无法使用学校模型", nil
			}
			apiKey, decryptErr := s.settingsCipher.decrypt(record.APIKey)
			if decryptErr != nil || strings.TrimSpace(apiKey) == "" {
				return nil, "学校 AI 密钥不可用，请联系管理员重新配置", nil
			}
			models := orderedModels(record.Models, record.DefaultModel, requestedModel)
			if requestedModel != "" {
				if !containsString(record.Models, requestedModel) {
					return nil, "", validationError("model", "所选模型未启用，请联系管理员在学校设置中开启")
				}
				models = []string{requestedModel}
			}
			if len(models) == 0 {
				return nil, "", validationError("model", "学校尚未启用任何 AI 模型")
			}
			candidates := make([]modelCandidate, 0, len(models))
			for _, model := range models {
				config := LLMConfig{
					Provider:     ProviderLLM,
					BaseURL:      record.BaseURL,
					APIKey:       apiKey,
					Model:        model,
					Timeout:      s.llmTimeout,
					Source:       "school",
					Models:       record.Models,
					DefaultModel: record.DefaultModel,
				}
				candidates = append(candidates, modelCandidate{
					Client: NewLLMClient(config.BaseURL, config.APIKey, config.Model, config.Timeout),
					Config: config,
					Key:    principal.SchoolID.String() + "|" + model,
				})
			}
			return candidates, "", nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, "", err
		}
	}
	if s.llm == nil || !s.llm.Enabled() {
		return nil, "", nil
	}
	if requestedModel != "" && requestedModel != s.llm.Model() {
		return nil, "", validationError("model", "所选模型未启用，请联系管理员在学校设置中开启")
	}
	config := LLMConfig{
		Provider:     ProviderLLM,
		BaseURL:      s.llm.baseURL,
		APIKey:       s.llm.apiKey,
		Model:        s.llm.Model(),
		Timeout:      s.llmTimeout,
		Source:       "env",
		Models:       []string{s.llm.Model()},
		DefaultModel: s.llm.Model(),
	}
	return []modelCandidate{{
		Client: s.llm,
		Config: config,
		Key:    principal.SchoolID.String() + "|" + s.llm.Model(),
	}}, "", nil
}

func orderedModels(models []string, defaultModel, requested string) []string {
	if requested != "" {
		return []string{requested}
	}
	ordered := make([]string, 0, len(models))
	seen := map[string]struct{}{}
	appendModel := func(model string) {
		model = strings.TrimSpace(model)
		if model == "" {
			return
		}
		if _, ok := seen[model]; ok {
			return
		}
		seen[model] = struct{}{}
		ordered = append(ordered, model)
	}
	appendModel(defaultModel)
	for _, model := range models {
		appendModel(model)
	}
	if len(ordered) > modelMaxCandidates {
		ordered = ordered[:modelMaxCandidates]
	}
	return ordered
}

// completeJSON 在候选模型之间执行重试、故障转移与熔断，返回值保证是合法 JSON。
func (s *Service) completeJSON(ctx context.Context, principal auth.Principal, requestedModel, system, user string) (string, LLMConfig, []string, error) {
	candidates, warning, err := s.resolveCandidates(ctx, principal, requestedModel)
	if err != nil {
		return "", LLMConfig{}, nil, err
	}
	warnings := []string{}
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if len(candidates) == 0 {
		return "", LLMConfig{Source: "local"}, warnings, nil
	}
	var lastErr error
	for _, candidate := range candidates {
		if !s.breaker().allow(candidate.Key, time.Now()) {
			warnings = append(warnings, candidate.Config.Model+" 暂时熔断，已跳过")
			continue
		}
		raw, callErr := s.completeWithCandidate(ctx, candidate, system, user)
		if callErr == nil {
			s.breaker().success(candidate.Key)
			return raw, candidate.Config, warnings, nil
		}
		lastErr = callErr
		if kind := errorKindOf(callErr); kind == ErrorKindConfig || kind == ErrorKindFatal {
			break
		}
		s.breaker().failure(candidate.Key, time.Now())
		warnings = append(warnings, fmt.Sprintf("%s 调用失败：%v", candidate.Config.Model, callErr))
	}
	if lastErr == nil {
		lastErr = errAllModelsOpen
	}
	return "", LLMConfig{}, warnings, fmt.Errorf("%w: %v", ErrModelUnavailable, lastErr)
}

func (s *Service) completeWithCandidate(ctx context.Context, candidate modelCandidate, system, user string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= modelMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		raw, err := candidate.Client.CompleteJSON(ctx, system, user)
		if err == nil {
			if json.Valid([]byte(raw)) {
				return raw, nil
			}
			repaired, repairErr := candidate.Client.CompleteJSON(ctx,
				"你是 JSON 修复助手，只输出严格合法的 JSON，不要任何解释。",
				"请把以下内容修复为严格合法的 JSON，保持字段与语义不变：\n"+raw)
			if repairErr == nil && json.Valid([]byte(repaired)) {
				return repaired, nil
			}
			err = &ProviderError{Kind: ErrorKindParse, Message: "模型返回内容不是合法 JSON"}
		}
		lastErr = err
		kind := errorKindOf(err)
		if kind == ErrorKindConfig || kind == ErrorKindFatal {
			return "", err
		}
		if attempt < modelMaxAttempts {
			delay := retryDelay(attempt)
			var providerErr *ProviderError
			if errors.As(err, &providerErr) && providerErr.RetryAfter > delay {
				delay = providerErr.RetryAfter
			}
			if err := sleepContext(ctx, delay); err != nil {
				return "", err
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("模型调用失败")
	}
	return "", lastErr
}

// chatTools 在单个候选模型上重试，成功即清零熔断计数。
func (s *Service) chatTools(ctx context.Context, candidate modelCandidate, messages []LLMMessage, tools []LLMTool) (LLMReply, error) {
	var lastErr error
	for attempt := 1; attempt <= modelMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return LLMReply{}, err
		}
		reply, err := candidate.Client.ChatTools(ctx, messages, tools)
		if err == nil {
			s.breaker().success(candidate.Key)
			return reply, nil
		}
		lastErr = err
		kind := errorKindOf(err)
		if kind == ErrorKindConfig || kind == ErrorKindFatal {
			return LLMReply{}, err
		}
		if attempt < modelMaxAttempts {
			delay := retryDelay(attempt)
			var providerErr *ProviderError
			if errors.As(err, &providerErr) && providerErr.RetryAfter > delay {
				delay = providerErr.RetryAfter
			}
			if err := sleepContext(ctx, delay); err != nil {
				return LLMReply{}, err
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("模型调用失败")
	}
	return LLMReply{}, lastErr
}

func errorKindOf(err error) ErrorKind {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) && providerErr.Kind != "" {
		return providerErr.Kind
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrorKindRetryable
	}
	return ErrorKindRetryable
}

// retryDelay 返回第 attempt 次失败后的退避时间：300ms、900ms，带 ±20% 抖动。
func retryDelay(attempt int) time.Duration {
	base := 300 * time.Millisecond
	for index := 1; index < attempt; index++ {
		base *= 3
	}
	jitter := 1 + (rand.Float64()*0.4 - 0.2)
	return time.Duration(float64(base) * jitter)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
