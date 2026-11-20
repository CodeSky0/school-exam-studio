package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/drive"
	"school-exam-studio/backend/internal/paper"
	"school-exam-studio/backend/internal/question"
)

// CloudDrive 是 AI 附件与云盘之间的桥接能力，由云盘服务实现。
type CloudDrive interface {
	ImportFromAI(ctx context.Context, principal auth.Principal, name string, data []byte) error
	ReadForAI(ctx context.Context, principal auth.Principal, id uuid.UUID) (drive.ReadFile, error)
}

type Options struct {
	QuestionService *question.Service
	PaperService    *paper.Service
	Repository      *Repository
	LLMBaseURL      string
	LLMAPIKey       string
	LLMModel        string
	LLMTimeout      time.Duration
	SettingsSecret  string
	UploadDir       string
	CloudDrive      CloudDrive
}

type Service struct {
	questions      *question.Service
	papers         *paper.Service
	repo           *Repository
	llm            *LLMClient
	local          *LocalEngine
	sanitizer      *question.Sanitizer
	settingsRepo   *Repository
	settingsCipher *settingsCipher
	llmTimeout     time.Duration
	attachments    *AttachmentStore
	cloudDrive     CloudDrive
	breakers       *breakerSet
}

func NewService(options Options) *Service {
	var store *AttachmentStore
	if strings.TrimSpace(options.UploadDir) != "" {
		store, _ = NewAttachmentStore(options.UploadDir)
	}
	return &Service{
		attachments:    store,
		questions:      options.QuestionService,
		papers:         options.PaperService,
		repo:           options.Repository,
		llm:            NewLLMClient(options.LLMBaseURL, options.LLMAPIKey, options.LLMModel, options.LLMTimeout),
		local:          NewLocalEngine(),
		sanitizer:      question.NewSanitizer(),
		settingsRepo:   options.Repository,
		settingsCipher: newSettingsCipher(options.SettingsSecret),
		llmTimeout:     options.LLMTimeout,
		cloudDrive:     options.CloudDrive,
		breakers:       newBreakerSet(defaultBreakerOpen, breakerThreshold),
	}
}

func (s *Service) Status(ctx context.Context, principal auth.Principal) Status {
	if err := validatePrincipal(principal); err != nil {
		return Status{Provider: ProviderLocal}
	}
	candidates, warning, err := s.resolveCandidates(ctx, principal, "")
	provider := ProviderLocal
	model := ""
	models := []string{}
	defaultModel := ""
	source := "local"
	health := make([]ModelHealth, 0, len(candidates))
	for _, candidate := range candidates {
		failures, openedUntil := s.breaker().state(candidate.Key)
		status := "healthy"
		retryAfter := 0
		if !openedUntil.IsZero() && time.Now().Before(openedUntil) {
			status = "open"
			retryAfter = int(time.Until(openedUntil).Seconds()) + 1
		} else if failures > 0 {
			status = "degraded"
		}
		health = append(health, ModelHealth{
			Model:             candidate.Config.Model,
			Status:            status,
			Failures:          failures,
			RetryAfterSeconds: retryAfter,
		})
	}
	if err == nil && len(candidates) > 0 {
		provider = ProviderLLM
		model = candidates[0].Config.Model
		models = candidates[0].Config.Models
		defaultModel = candidates[0].Config.DefaultModel
		source = candidates[0].Config.Source
	} else if warning != "" {
		provider = ProviderFallback
		source = "local"
	}
	fallbackPolicy := "error"
	if s.allowLocalFallback() {
		fallbackPolicy = "local"
	}
	return Status{
		Provider:       provider,
		Model:          model,
		LLMEnabled:     provider == ProviderLLM,
		Models:         models,
		DefaultModel:   defaultModel,
		Source:         source,
		Warning:        warning,
		FallbackPolicy: fallbackPolicy,
		ModelsHealth:   health,
		Limits: Limits{
			MaxAnalyzeChars: DefaultMaxAnalyzeChars,
			MaxGenerate:     DefaultMaxGenerate,
			MaxBatch:        DefaultMaxBatch,
		},
		Capabilities: []string{
			"题目分析、分类、知识点与标签建议",
			"答案与解析生成",
			"题目变式生成",
			"知识点驱动出题",
			"题库体检与重复题检测",
			"AI Agent 批量整理题库与智能组卷",
			"AI Agent 多轮对话、持久化会话与工具调用时间线",
			"PDF/Word/图片文档阅读与智能切题入库",
			"图片由多模态模型直接阅读，不依赖 OCR sidecar",
			"内置技能目录与外部 Agent 工具发现",
		},
		Safety: []string{
			"AI 只自动修改草稿，已发布题目只读分析",
			"教师只能操作自己的草稿，管理员可操作本校草稿",
			"所有变更写入审计日志",
			"AI 生成内容标记为待人工复核",
			"对话附件保存在配置的上传目录，图片仅在本轮对话中作为多模态输入发送给模型",
			"模型在工具执行后中断时不会重复执行已完成的写操作",
		},
	}
}

const connectionTestPrompt = `只输出 JSON：{"ok":true}`

// TestConnection 用一次最小请求验证学校当前保存的模型是否可用，便于管理员定位配置问题。
func (s *Service) TestConnection(ctx context.Context, principal auth.Principal, model string) (ConnectionTestResult, error) {
	if principal.Role != auth.RoleAdmin {
		return ConnectionTestResult{}, ErrForbidden
	}
	client, config, warning, err := s.resolveLLM(ctx, principal, model)
	if err != nil {
		var validation *ValidationError
		if errors.As(err, &validation) {
			return ConnectionTestResult{OK: false, Provider: ProviderLocal, Message: validation.Violation.Message}, nil
		}
		return ConnectionTestResult{}, err
	}
	if client == nil {
		message := warning
		if message == "" {
			message = "当前使用本地确定性引擎，尚未配置可用模型"
		}
		return ConnectionTestResult{OK: false, Provider: ProviderLocal, Source: "local", Message: message}, nil
	}
	start := time.Now()
	_, callErr := client.CompleteJSON(ctx, "你是连接测试助手。", connectionTestPrompt)
	latency := time.Since(start).Milliseconds()
	result := ConnectionTestResult{
		OK:        callErr == nil,
		Provider:  config.Provider,
		Model:     config.Model,
		Source:    config.Source,
		LatencyMS: latency,
		Message:   "模型连接正常",
	}
	if callErr != nil {
		result.Message = callErr.Error()
	}
	return result, nil
}

func (s *Service) Analyze(ctx context.Context, principal auth.Principal, input AnalyzeInput) (Analysis, error) {
	if err := validatePrincipal(principal); err != nil {
		return Analysis{}, err
	}
	if err := s.validateAnalyzeInput(&input); err != nil {
		return Analysis{}, err
	}
	local := s.local.Analyze(input)
	raw, config, warnings, err := s.completeJSON(ctx, principal, input.Model, analyzeSystemPrompt, analyzeUserPrompt(input))
	if err != nil {
		if !s.allowLocalFallback() {
			return Analysis{}, err
		}
		local.Provider = ProviderFallback
		local.Warnings = append(local.Warnings, "大模型调用失败，已按 AI_ALLOW_LOCAL_FALLBACK 配置降级为本地引擎", "降级原因："+err.Error())
		local.Warnings = append(local.Warnings, warnings...)
		return local, nil
	}
	if config.Source == "local" {
		local.Warnings = append(local.Warnings, warnings...)
		return local, nil
	}
	var response llmAnalysisResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return Analysis{}, fmt.Errorf("%w: 模型返回内容无法解析", ErrModelUnavailable)
	}
	analysis := s.normalizeAnalysis(response, input, local, config.Model)
	analysis.Warnings = append(analysis.Warnings, warnings...)
	return analysis, nil
}

func (s *Service) AnalyzeQuestion(ctx context.Context, principal auth.Principal, id uuid.UUID, apply bool) (ApplyResult, error) {
	return s.analyzeQuestion(ctx, principal, id, apply, "", "")
}

func (s *Service) analyzeQuestion(ctx context.Context, principal auth.Principal, id uuid.UUID, apply bool, runID string, model string) (ApplyResult, error) {
	existing, err := s.questions.Get(ctx, principal, id)
	if err != nil {
		return ApplyResult{}, err
	}
	input := analyzeInputFromQuestion(existing)
	input.Model = model
	analysis, err := s.Analyze(ctx, principal, input)
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{Analysis: analysis}
	if !apply {
		return result, nil
	}
	patch := buildApplyPatch(existing, analysis, &result.Analysis)
	result.Applied = true
	updated, err := s.questions.ApplyAI(ctx, principal, id, patch)
	if err != nil {
		return ApplyResult{}, err
	}
	result.Question = updated
	if s.repo != nil {
		_ = s.repo.CreateAudit(ctx, principal, AuditLog{
			Action:     "analyze_and_apply",
			TargetType: "question",
			TargetID:   id.String(),
			RunID:      runID,
			Detail: map[string]any{
				"provider":         analysis.Provider,
				"model":            analysis.Model,
				"category":         updated.Category,
				"tags":             updated.Tags,
				"knowledge_points": updated.KnowledgePoints,
			},
		})
	}
	return result, nil
}

func (s *Service) Generate(ctx context.Context, principal auth.Principal, input GenerateInput) (GenerateResult, error) {
	return s.generate(ctx, principal, input, "")
}

func (s *Service) generate(ctx context.Context, principal auth.Principal, input GenerateInput, runID string) (GenerateResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return GenerateResult{}, err
	}
	if err := s.validateGenerateInput(principal, &input); err != nil {
		return GenerateResult{}, err
	}
	result := GenerateResult{Warnings: []string{}}
	raw, config, warnings, err := s.completeJSON(ctx, principal, input.Model, generateSystemPrompt, generateUserPrompt(input))
	result.Warnings = append(result.Warnings, warnings...)
	if err != nil {
		if !s.allowLocalFallback() {
			return GenerateResult{}, err
		}
		items, localWarnings := s.local.Generate(input)
		result.Items = items
		result.Provider = ProviderFallback
		result.Warnings = append(result.Warnings, "大模型出题失败，已按 AI_ALLOW_LOCAL_FALLBACK 配置降级为本地引擎")
		result.Warnings = append(result.Warnings, localWarnings...)
	} else if config.Source == "local" {
		items, localWarnings := s.local.Generate(input)
		result.Items = items
		result.Provider = ProviderLocal
		result.Warnings = append(result.Warnings, localWarnings...)
	} else {
		var response llmItemsResponse
		if err := json.Unmarshal([]byte(raw), &response); err != nil {
			return GenerateResult{}, fmt.Errorf("%w: 模型返回内容无法解析", ErrModelUnavailable)
		}
		result.Items = s.normalizeGenerated(response.Items, input)
		if len(result.Items) == 0 {
			if !s.allowLocalFallback() {
				return GenerateResult{}, fmt.Errorf("%w: 模型返回的题目未通过校验", ErrModelUnavailable)
			}
			items, localWarnings := s.local.Generate(input)
			result.Items = items
			result.Provider = ProviderFallback
			result.Warnings = append(result.Warnings, "模型返回的题目未通过校验，已按配置降级为本地引擎")
			result.Warnings = append(result.Warnings, localWarnings...)
		} else {
			result.Provider = ProviderLLM
			result.Model = config.Model
			if len(result.Items) < len(response.Items) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("大模型返回的 %d 道题中有 %d 道未通过校验，已过滤", len(response.Items), len(response.Items)-len(result.Items)))
			}
		}
	}
	if input.Persist {
		persisted, ids, warnings := s.persistGenerated(ctx, principal, input.Subject, input.Grade, result.Items, uuid.Nil, "generate_questions", runID, result.Model)
		result.Persisted = persisted
		result.QuestionIDs = ids
		result.Warnings = append(result.Warnings, warnings...)
	}
	return result, nil
}

func (s *Service) GenerateVariants(ctx context.Context, principal auth.Principal, id uuid.UUID, input VariantInput) (VariantResult, error) {
	return s.generateVariants(ctx, principal, id, input, "")
}

func (s *Service) generateVariants(ctx context.Context, principal auth.Principal, id uuid.UUID, input VariantInput, runID string) (VariantResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return VariantResult{}, err
	}
	if input.Count < 1 || input.Count > DefaultMaxGenerate {
		return VariantResult{}, validationError("count", fmt.Sprintf("变式数量必须为1-%d", DefaultMaxGenerate))
	}
	if input.Strategy == "" {
		input.Strategy = "numbers"
	}
	switch input.Strategy {
	case "numbers", "context", "reverse", "difficulty":
	default:
		return VariantResult{}, validationError("strategy", "变式策略不受支持")
	}
	if input.DifficultyDelta < -2 || input.DifficultyDelta > 2 {
		return VariantResult{}, validationError("difficulty_delta", "难度变化必须在-2到2之间")
	}
	source, err := s.questions.Get(ctx, principal, id)
	if err != nil {
		return VariantResult{}, err
	}
	snapshot := toSnapshot(source)
	result := VariantResult{Warnings: []string{}}
	difficulty := source.Difficulty + input.DifficultyDelta
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 5 {
		difficulty = 5
	}
	generateInput := GenerateInput{
		Subject:         source.Subject,
		Grade:           source.Grade,
		QuestionType:    source.QuestionType,
		Difficulty:      difficulty,
		Category:        source.Category,
		KnowledgePoints: source.KnowledgePoints,
		Tags:            source.Tags,
		Count:           input.Count,
	}
	raw, config, warnings, err := s.completeJSON(ctx, principal, input.Model, variantSystemPrompt, variantUserPrompt(snapshot, input))
	result.Warnings = append(result.Warnings, warnings...)
	if err != nil {
		if !s.allowLocalFallback() {
			return VariantResult{}, err
		}
		items, localWarnings := s.local.Variants(snapshot, input)
		result.Items = items
		result.Provider = ProviderFallback
		result.Warnings = append(result.Warnings, "大模型变式生成失败，已按 AI_ALLOW_LOCAL_FALLBACK 配置降级为本地引擎")
		result.Warnings = append(result.Warnings, localWarnings...)
	} else if config.Source == "local" {
		items, localWarnings := s.local.Variants(snapshot, input)
		result.Items = items
		result.Provider = ProviderLocal
		result.Warnings = append(result.Warnings, localWarnings...)
	} else {
		var response llmItemsResponse
		if err := json.Unmarshal([]byte(raw), &response); err != nil {
			return VariantResult{}, fmt.Errorf("%w: 模型返回内容无法解析", ErrModelUnavailable)
		}
		result.Items = s.normalizeGenerated(response.Items, generateInput)
		for index := range result.Items {
			result.Items[index].ReviewRequired = true
			result.Items[index].Rationale = firstNonEmpty(result.Items[index].Rationale, "基于原题生成的变式")
		}
		if len(result.Items) == 0 {
			if !s.allowLocalFallback() {
				return VariantResult{}, fmt.Errorf("%w: 模型返回的变式未通过校验", ErrModelUnavailable)
			}
			items, localWarnings := s.local.Variants(snapshot, input)
			result.Items = items
			result.Provider = ProviderFallback
			result.Warnings = append(result.Warnings, "模型返回的变式未通过校验，已按配置降级为本地引擎")
			result.Warnings = append(result.Warnings, localWarnings...)
		} else {
			result.Provider = ProviderLLM
			result.Model = config.Model
			if len(result.Items) < len(response.Items) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("大模型返回的 %d 道变式中 %d 道未通过校验，已过滤", len(response.Items), len(response.Items)-len(result.Items)))
			}
		}
	}
	if input.Persist {
		persisted, ids, warnings := s.persistGenerated(ctx, principal, source.Subject, source.Grade, result.Items, source.ID, "generate_variants", runID, result.Model)
		result.Persisted = persisted
		result.QuestionIDs = ids
		result.Warnings = append(result.Warnings, warnings...)
	}
	return result, nil
}

func (s *Service) AuditBank(ctx context.Context, principal auth.Principal, limit int) (AuditResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return AuditResult{}, err
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}
	items, err := s.questions.Snapshot(ctx, principal, limit)
	if err != nil {
		return AuditResult{}, err
	}
	snapshots := make([]QuestionSnapshot, 0, len(items))
	for _, item := range items {
		snapshots = append(snapshots, toSnapshot(item))
	}
	return s.local.Audit(snapshots), nil
}

func (s *Service) ListRuns(ctx context.Context, principal auth.Principal, limit int) ([]AgentRun, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return []AgentRun{}, nil
	}
	return s.repo.ListRuns(ctx, principal, limit)
}

func (s *Service) GetRun(ctx context.Context, principal auth.Principal, id uuid.UUID) (AgentRun, error) {
	if err := validatePrincipal(principal); err != nil {
		return AgentRun{}, err
	}
	if s.repo == nil {
		return AgentRun{}, ErrNotFound
	}
	return s.repo.GetRun(ctx, principal, id)
}

func (s *Service) ListAudits(ctx context.Context, principal auth.Principal, limit int) ([]AuditLog, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if principal.Role != auth.RoleAdmin {
		return nil, ErrForbidden
	}
	if s.repo == nil {
		return []AuditLog{}, nil
	}
	return s.repo.ListAudits(ctx, principal, limit)
}

func (s *Service) validateAnalyzeInput(input *AnalyzeInput) error {
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" {
		return validationError("content", "题干不能为空")
	}
	if len([]rune(input.Content)) > DefaultMaxAnalyzeChars {
		return validationError("content", "题干长度超过限制")
	}
	if len(input.Options) > 4 {
		return validationError("options", "选项最多4个")
	}
	if input.QuestionType != "" && !question.ValidType(input.QuestionType) {
		return validationError("question_type", "题型不受支持")
	}
	if input.Difficulty < 0 || input.Difficulty > 5 {
		return validationError("difficulty", "难度必须为1-5")
	}
	return nil
}

func (s *Service) validateGenerateInput(principal auth.Principal, input *GenerateInput) error {
	input.Subject = strings.TrimSpace(input.Subject)
	input.Grade = strings.TrimSpace(input.Grade)
	input.Category = strings.TrimSpace(input.Category)
	input.QuestionType = strings.TrimSpace(input.QuestionType)
	input.KnowledgePoints = uniqueStrings(input.KnowledgePoints)
	input.Tags = uniqueStrings(input.Tags)
	if principal.Role == auth.RoleTeacher {
		input.Subject = principal.Subject
	}
	if input.Subject == "" {
		return validationError("subject", "请提供学科")
	}
	if input.Grade == "" {
		return validationError("grade", "请提供年级")
	}
	if input.QuestionType != "" && !question.ValidType(input.QuestionType) {
		return validationError("question_type", "题型不受支持")
	}
	if input.Difficulty < 0 || input.Difficulty > 5 {
		return validationError("difficulty", "难度必须为1-5")
	}
	if input.Count <= 0 {
		input.Count = 3
	}
	if input.Count > DefaultMaxGenerate {
		return validationError("count", fmt.Sprintf("单次出题数量不能超过%d", DefaultMaxGenerate))
	}
	if len(input.KnowledgePoints) == 0 && input.Category == "" && len(input.Tags) == 0 && strings.TrimSpace(input.Reference) == "" {
		return validationError("knowledge_points", "请至少提供知识点、标签、分类或参考题干")
	}
	return nil
}

func (s *Service) normalizeAnalysis(response llmAnalysisResponse, input AnalyzeInput, fallback Analysis, model string) Analysis {
	subject := firstNonEmpty(input.Subject, response.Subject, fallback.Subject)
	grade := firstNonEmpty(input.Grade, response.Grade, fallback.Grade)
	questionType := input.QuestionType
	if !question.ValidType(questionType) {
		questionType = response.QuestionType
	}
	if !question.ValidType(questionType) {
		questionType = fallback.QuestionType
	}
	difficulty := response.Difficulty
	if difficulty < 1 || difficulty > 5 {
		difficulty = fallback.Difficulty
	}
	category := firstNonEmpty(input.Category, response.Category, fallback.Category)
	if len([]rune(category)) > 30 {
		category = string([]rune(category)[:30])
	}
	knowledgePoints := uniqueStrings(append(append([]string{}, input.KnowledgePoints...), response.KnowledgePoints...))
	if len(knowledgePoints) > 6 {
		knowledgePoints = knowledgePoints[:6]
	}
	if len(knowledgePoints) == 0 {
		knowledgePoints = fallback.KnowledgePoints
	}
	tags := uniqueStrings(append(append([]string{}, input.Tags...), response.Tags...))
	if len(tags) == 0 {
		tags = fallback.Tags
	}
	tags = dropDuplicateMetadataTags(tags, category, knowledgePoints)
	if len(tags) > 10 {
		tags = tags[:10]
	}
	answer := strings.TrimSpace(input.Answer)
	if answer == "" {
		answer = strings.TrimSpace(response.Answer)
	}
	explanation := strings.TrimSpace(input.Explanation)
	if explanation == "" {
		explanation = strings.TrimSpace(response.Explanation)
	}
	if explanation != "" {
		// 即使清洗后为空（例如只包含 script 标签），也必须采用清洗结果，避免把未过滤的 HTML 返回给前端。
		if sanitized, err := s.sanitizer.NormalizeExplanation(explanation); err == nil {
			explanation = sanitized
		} else {
			explanation = ""
		}
	}
	if strings.TrimSpace(explanation) == "" {
		explanation = fallback.Explanation
	}
	confidence := response.Confidence
	if confidence < 0 || confidence > 1 {
		confidence = 0.75
	}
	return Analysis{
		Subject:         subject,
		Grade:           grade,
		QuestionType:    questionType,
		Category:        category,
		Difficulty:      difficulty,
		Tags:            tags,
		KnowledgePoints: knowledgePoints,
		Answer:          answer,
		Explanation:     explanation,
		Confidence:      confidence,
		Provider:        ProviderLLM,
		Model:           model,
		ReviewRequired:  true,
		Rationale:       firstNonEmpty(response.Rationale, "大模型分析结果，建议人工复核"),
		Warnings:        []string{"大模型结果建议人工复核，尤其是答案与解析"},
	}
}

func (s *Service) normalizeGenerated(items []llmItem, input GenerateInput) []GeneratedQuestion {
	result := make([]GeneratedQuestion, 0, len(items))
	for _, item := range items {
		if len(result) >= input.Count {
			break
		}
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		questionType := item.QuestionType
		if !question.ValidType(questionType) {
			questionType = input.QuestionType
		}
		if !question.ValidType(questionType) {
			questionType = question.TypeSingleChoice
		}
		difficulty := item.Difficulty
		if difficulty < 1 || difficulty > 5 {
			difficulty = firstPositive(input.Difficulty, 3)
		}
		category := firstNonEmpty(item.Category, input.Category, "综合应用")
		if len([]rune(category)) > 30 {
			category = string([]rune(category)[:30])
		}
		knowledgePoints := uniqueStrings(append(append([]string{}, input.KnowledgePoints...), item.KnowledgePoints...))
		if len(knowledgePoints) > 6 {
			knowledgePoints = knowledgePoints[:6]
		}
		tags := uniqueStrings(append(append([]string{}, input.Tags...), item.Tags...))
		tags = dropDuplicateMetadataTags(tags, category, knowledgePoints)
		if len(tags) > 10 {
			tags = tags[:10]
		}
		generated := GeneratedQuestion{
			Content:         item.Content,
			Options:         item.Options,
			Answer:          strings.TrimSpace(item.Answer),
			Explanation:     item.Explanation,
			QuestionType:    questionType,
			Difficulty:      difficulty,
			Category:        category,
			Tags:            tags,
			KnowledgePoints: knowledgePoints,
			Rationale:       item.Rationale,
			ReviewRequired:  true,
			Provider:        ProviderLLM,
		}
		if sanitized, err := s.sanitizer.Normalize(question.Input{
			Subject:         input.Subject,
			Grade:           input.Grade,
			QuestionType:    generated.QuestionType,
			Difficulty:      generated.Difficulty,
			Tags:            generated.Tags,
			Category:        generated.Category,
			KnowledgePoints: generated.KnowledgePoints,
			Content:         generated.Content,
			Options:         toQuestionOptions(generated.Options),
			Answer:          generated.Answer,
			Explanation:     generated.Explanation,
		}); err == nil {
			generated.Content = sanitized.Content
			generated.Options = fromQuestionOptions(sanitized.Options)
			generated.Answer = sanitized.Answer
			generated.Explanation = sanitized.Explanation
			generated.Category = sanitized.Category
			generated.Tags = sanitized.Tags
			generated.KnowledgePoints = sanitized.KnowledgePoints
			generated.QuestionType = sanitized.QuestionType
		} else {
			continue
		}
		// 非选择题 Options 为空时必须序列化为 [] 而不是 null，否则前端 schema 解析会失败。
		if generated.Options == nil {
			generated.Options = []Option{}
		}
		result = append(result, generated)
	}
	return result
}

func (s *Service) persistGenerated(ctx context.Context, principal auth.Principal, subject, grade string, items []GeneratedQuestion, sourceID uuid.UUID, action string, runID string, model string) ([]GeneratedQuestion, []string, []string) {
	persisted := make([]GeneratedQuestion, 0, len(items))
	ids := make([]string, 0, len(items))
	warnings := []string{}
	for index, item := range items {
		if principal.Role == auth.RoleTeacher {
			subject = principal.Subject
		}
		meta := map[string]any{
			"provider":        item.Provider,
			"model":           model,
			"review_required": true,
			"rationale":       item.Rationale,
			"generated_at":    time.Now().UTC().Format(time.RFC3339),
		}
		if sourceID != uuid.Nil {
			meta["source_question_id"] = sourceID.String()
		}
		created, err := s.questions.Create(ctx, principal, question.Input{
			Subject:          firstNonEmpty(subject, "通用"),
			Grade:            grade,
			QuestionType:     item.QuestionType,
			Difficulty:       item.Difficulty,
			Tags:             item.Tags,
			Category:         item.Category,
			KnowledgePoints:  item.KnowledgePoints,
			Content:          item.Content,
			Options:          toQuestionOptions(item.Options),
			Answer:           item.Answer,
			Explanation:      item.Explanation,
			SourceQuestionID: sourceID,
			AIGenerated:      true,
			AIMeta:           meta,
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("第%d道题保存失败：%s", index+1, userFacingQuestionError(err)))
			continue
		}
		persisted = append(persisted, item)
		ids = append(ids, created.ID.String())
		if s.repo != nil {
			_ = s.repo.CreateAudit(ctx, principal, AuditLog{
				Action:     action,
				TargetType: "question",
				TargetID:   created.ID.String(),
				RunID:      runID,
				Detail: map[string]any{
					"provider":           item.Provider,
					"subject":            created.Subject,
					"source_question_id": created.SourceQuestionID,
					"review_required":    true,
				},
			})
		}
	}
	return persisted, ids, uniqueStrings(warnings)
}

func analyzeInputFromQuestion(item question.Question) AnalyzeInput {
	return AnalyzeInput{
		Subject:         item.Subject,
		Grade:           item.Grade,
		QuestionType:    item.QuestionType,
		Difficulty:      item.Difficulty,
		Tags:            item.Tags,
		Category:        item.Category,
		KnowledgePoints: item.KnowledgePoints,
		Content:         item.Content,
		Options:         fromQuestionOptions(item.Options),
		Answer:          item.Answer,
		Explanation:     item.Explanation,
	}
}

func buildApplyPatch(existing question.Question, analysis Analysis, effective *Analysis) question.AIApplyPatch {
	patch := question.AIApplyPatch{
		Provider:       analysis.Provider,
		Model:          analysis.Model,
		Confidence:     analysis.Confidence,
		Rationale:      analysis.Rationale,
		ReviewRequired: true,
	}
	if analysis.Category != "" {
		patch.Category = &analysis.Category
	}
	if len(analysis.KnowledgePoints) > 0 {
		values := uniqueStrings(analysis.KnowledgePoints)
		patch.KnowledgePoints = &values
	}
	if len(analysis.Tags) > 0 {
		values := uniqueStrings(append(append([]string{}, existing.Tags...), analysis.Tags...))
		if len(values) > 10 {
			values = values[:10]
		}
		patch.Tags = &values
	}
	if analysis.Difficulty >= 1 && analysis.Difficulty <= 5 {
		value := analysis.Difficulty
		patch.Difficulty = &value
	}
	if strings.TrimSpace(existing.Answer) == "" && strings.TrimSpace(analysis.Answer) != "" {
		if value, err := question.ValidateAnswer(existing.QuestionType, existing.Options, analysis.Answer); err == nil {
			patch.Answer = &value
		} else {
			effective.Warnings = append(effective.Warnings, "AI 建议答案不符合题型约束，已跳过")
		}
	}
	if strings.TrimSpace(existing.Explanation) == "" && strings.TrimSpace(analysis.Explanation) != "" {
		value := analysis.Explanation
		patch.Explanation = &value
	}
	return patch
}

func toSnapshot(item question.Question) QuestionSnapshot {
	sourceID := ""
	if item.SourceQuestionID != nil {
		sourceID = item.SourceQuestionID.String()
	}
	return QuestionSnapshot{
		ID:               item.ID.String(),
		CreatorID:        item.CreatorID.String(),
		Subject:          item.Subject,
		Grade:            item.Grade,
		QuestionType:     item.QuestionType,
		Difficulty:       item.Difficulty,
		Tags:             item.Tags,
		Category:         item.Category,
		KnowledgePoints:  item.KnowledgePoints,
		Content:          item.Content,
		Options:          fromQuestionOptions(item.Options),
		Answer:           item.Answer,
		Explanation:      item.Explanation,
		Status:           item.Status,
		AIGenerated:      item.AIGenerated,
		ContentHash:      item.ContentHash,
		SourceQuestionID: sourceID,
		CreatedAt:        item.CreatedAt,
	}
}

func fromQuestionOptions(options []question.Option) []Option {
	result := make([]Option, 0, len(options))
	for _, option := range options {
		result = append(result, Option{Key: option.Key, Content: option.Content})
	}
	return result
}

func validatePrincipal(principal auth.Principal) error {
	if principal.UserID == uuid.Nil || principal.SchoolID == uuid.Nil {
		return ErrForbidden
	}
	if principal.Role != auth.RoleTeacher && principal.Role != auth.RoleAdmin {
		return ErrForbidden
	}
	return nil
}

func userFacingQuestionError(err error) string {
	var validation *question.ValidationError
	if errors.As(err, &validation) {
		if validation.Violation.Message != "" {
			return validation.Violation.Message
		}
		return "题目校验未通过"
	}
	switch {
	case errors.Is(err, question.ErrSubjectMismatch):
		return "学科与教师账户不一致"
	case errors.Is(err, question.ErrForbidden):
		return "没有权限"
	default:
		return "题目校验未通过"
	}
}

func firstPositive(value, fallback int) int {
	if value >= 1 && value <= 5 {
		return value
	}
	return fallback
}
