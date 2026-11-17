package ai

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"school-exam-studio/backend/internal/auth"
)

const (
	AIProviderOpenAI   = "openai"
	AIProviderDeepSeek = "deepseek"
	AIProviderQwen     = "qwen"
	AIProviderZhipu    = "zhipu"
	AIProviderMoonshot = "moonshot"
	AIProviderCustom   = "custom"

	settingsCipherPrefix = "v1:"
	maxAIModels          = 20
	maxAIModelLength     = 120
	maxAIBaseURLLength   = 500
)

var aiProviders = map[string]struct{}{
	AIProviderOpenAI: {}, AIProviderDeepSeek: {}, AIProviderQwen: {},
	AIProviderZhipu: {}, AIProviderMoonshot: {}, AIProviderCustom: {},
}

// LLMConfig 描述一次 AI 调用实际使用的模型连接，来源于学校共享配置或服务器环境变量。
type LLMConfig struct {
	Provider     string
	BaseURL      string
	APIKey       string
	Model        string
	Timeout      time.Duration
	Source       string
	Models       []string
	DefaultModel string
}

type SchoolAISettingsInput struct {
	Provider     string   `json:"provider"`
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	Models       []string `json:"models"`
	DefaultModel string   `json:"default_model"`
	Enabled      bool     `json:"enabled"`
	ClearAPIKey  bool     `json:"clear_api_key"`
}

type SchoolAISettingsView struct {
	Provider     string     `json:"provider"`
	BaseURL      string     `json:"base_url"`
	APIKeyHint   string     `json:"api_key_hint"`
	HasAPIKey    bool       `json:"has_api_key"`
	Models       []string   `json:"models"`
	DefaultModel string     `json:"default_model"`
	Enabled      bool       `json:"enabled"`
	Configured   bool       `json:"configured"`
	Source       string     `json:"source"`
	Warning      string     `json:"warning,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

type schoolAISettingRecord struct {
	Provider     string
	BaseURL      string
	APIKey       string
	Models       []string
	DefaultModel string
	Enabled      bool
	UpdatedAt    time.Time
}

type settingsCipher struct {
	aead cipher.AEAD
}

func newSettingsCipher(secret string) *settingsCipher {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	key := sha256.Sum256([]byte("school-exam-studio:ai-settings:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	return &settingsCipher{aead: aead}
}

func (c *settingsCipher) encrypt(plain string) (string, error) {
	if c == nil {
		return "", errors.New("ai settings cipher is not configured")
	}
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return settingsCipherPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c *settingsCipher) decrypt(value string) (string, error) {
	if c == nil {
		return "", errors.New("ai settings cipher is not configured")
	}
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, settingsCipherPrefix) {
		return "", errors.New("unsupported ai settings ciphertext")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, settingsCipherPrefix))
	if err != nil {
		return "", err
	}
	nonceSize := c.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("invalid ai settings ciphertext")
	}
	plain, err := c.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *Service) GetSchoolAISettings(ctx context.Context, principal auth.Principal) (SchoolAISettingsView, error) {
	if principal.Role != auth.RoleAdmin {
		return SchoolAISettingsView{}, ErrForbidden
	}
	if s.settingsRepo == nil {
		return s.envSettingsView(), nil
	}
	record, err := s.settingsRepo.GetSchoolAISetting(ctx, principal.SchoolID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return s.envSettingsView(), nil
		}
		return SchoolAISettingsView{}, err
	}
	return s.settingsView(record), nil
}

func (s *Service) UpdateSchoolAISettings(ctx context.Context, principal auth.Principal, input SchoolAISettingsInput) (SchoolAISettingsView, error) {
	if principal.Role != auth.RoleAdmin {
		return SchoolAISettingsView{}, ErrForbidden
	}
	if s.settingsRepo == nil || s.settingsCipher == nil {
		return SchoolAISettingsView{}, validationError("settings", "服务器未配置 AI 设置加密密钥，无法保存学校模型配置")
	}
	normalized, err := normalizeSchoolAISettings(input)
	if err != nil {
		return SchoolAISettingsView{}, err
	}
	existing, getErr := s.settingsRepo.GetSchoolAISetting(ctx, principal.SchoolID)
	if getErr != nil && !errors.Is(getErr, ErrNotFound) {
		return SchoolAISettingsView{}, getErr
	}
	record := schoolAISettingRecord{
		Provider:     normalized.Provider,
		BaseURL:      normalized.BaseURL,
		Models:       normalized.Models,
		DefaultModel: normalized.DefaultModel,
		Enabled:      normalized.Enabled,
	}
	if getErr == nil {
		record.APIKey = existing.APIKey
	}
	if strings.TrimSpace(normalized.APIKey) != "" {
		encrypted, encryptErr := s.settingsCipher.encrypt(strings.TrimSpace(normalized.APIKey))
		if encryptErr != nil {
			return SchoolAISettingsView{}, encryptErr
		}
		record.APIKey = encrypted
	} else if normalized.ClearAPIKey {
		record.APIKey = ""
	}
	if record.Enabled {
		if record.BaseURL == "" {
			return SchoolAISettingsView{}, validationError("base_url", "启用全校 AI 前必须填写接口地址")
		}
		if record.APIKey == "" {
			return SchoolAISettingsView{}, validationError("api_key", "启用全校 AI 前必须填写 API Key")
		}
		if len(record.Models) == 0 {
			return SchoolAISettingsView{}, validationError("models", "启用全校 AI 前至少启用一个模型")
		}
	}
	saved, err := s.settingsRepo.SaveSchoolAISetting(ctx, principal, record)
	if err != nil {
		return SchoolAISettingsView{}, err
	}
	s.saveAuditForSettings(ctx, principal, "update_school_ai_settings", map[string]any{
		"provider":      saved.Provider,
		"models":        saved.Models,
		"default_model": saved.DefaultModel,
		"enabled":       saved.Enabled,
		"api_key_set":   saved.APIKey != "",
	})
	return s.settingsView(saved), nil
}

func (s *Service) resolveLLM(ctx context.Context, principal auth.Principal, requestedModel string) (*LLMClient, LLMConfig, string, error) {
	candidates, warning, err := s.resolveCandidates(ctx, principal, requestedModel)
	if err != nil {
		return nil, LLMConfig{}, "", err
	}
	if len(candidates) == 0 {
		return nil, LLMConfig{Source: "local"}, warning, nil
	}
	return candidates[0].Client, candidates[0].Config, warning, nil
}

func (s *Service) envSettingsView() SchoolAISettingsView {
	view := SchoolAISettingsView{Provider: AIProviderCustom, Models: []string{}, Source: "local"}
	if s.llm != nil && s.llm.Enabled() {
		view.Models = []string{s.llm.Model()}
		view.DefaultModel = s.llm.Model()
		view.Source = "env"
	}
	return view
}

func (s *Service) settingsView(record schoolAISettingRecord) SchoolAISettingsView {
	view := SchoolAISettingsView{
		Provider:     record.Provider,
		BaseURL:      record.BaseURL,
		Models:       record.Models,
		DefaultModel: record.DefaultModel,
		Enabled:      record.Enabled,
		Configured:   true,
		Source:       "local",
		UpdatedAt:    &record.UpdatedAt,
	}
	if record.Enabled {
		view.Source = "school"
	}
	if record.APIKey != "" {
		view.HasAPIKey = true
		view.APIKeyHint = "已保存"
		if s.settingsCipher != nil {
			if apiKey, err := s.settingsCipher.decrypt(record.APIKey); err == nil {
				if runes := []rune(apiKey); len(runes) >= 4 {
					view.APIKeyHint = "••••" + string(runes[len(runes)-4:])
				}
			} else {
				view.Warning = "已保存的 API Key 无法解密，请重新输入密钥。"
			}
		}
	}
	return view
}

func normalizeSchoolAISettings(input SchoolAISettingsInput) (SchoolAISettingsInput, error) {
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	if input.Provider == "" {
		input.Provider = AIProviderCustom
	}
	if _, ok := aiProviders[input.Provider]; !ok {
		return SchoolAISettingsInput{}, validationError("provider", "请选择受支持的模型厂商")
	}
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if input.BaseURL != "" {
		if len(input.BaseURL) > maxAIBaseURLLength {
			return SchoolAISettingsInput{}, validationError("base_url", "接口地址不能超过500个字符")
		}
		parsed, err := url.Parse(input.BaseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
			return SchoolAISettingsInput{}, validationError("base_url", "接口地址必须是有效的 HTTP(S) 地址")
		}
	}
	models := make([]string, 0, len(input.Models))
	seen := map[string]struct{}{}
	for _, model := range input.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if utf8.RuneCountInString(model) > maxAIModelLength {
			return SchoolAISettingsInput{}, validationError("models", "模型名称不能超过120个字符")
		}
		for _, r := range model {
			if unicode.IsControl(r) {
				return SchoolAISettingsInput{}, validationError("models", "模型名称不能包含控制字符")
			}
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
		if len(models) > maxAIModels {
			return SchoolAISettingsInput{}, validationError("models", "最多启用20个模型")
		}
	}
	input.Models = models
	input.DefaultModel = strings.TrimSpace(input.DefaultModel)
	if input.DefaultModel == "" && len(models) > 0 {
		input.DefaultModel = models[0]
	}
	if input.DefaultModel != "" {
		if _, ok := seen[input.DefaultModel]; !ok {
			return SchoolAISettingsInput{}, validationError("default_model", "默认模型必须来自已启用的模型列表")
		}
	}
	input.APIKey = strings.TrimSpace(input.APIKey)
	if len(input.APIKey) > 500 {
		return SchoolAISettingsInput{}, validationError("api_key", "API Key 不能超过500个字符")
	}
	return input, nil
}

func chooseAIModel(models []string, defaultModel, requested string) (string, error) {
	if requested == "" {
		if defaultModel != "" {
			return defaultModel, nil
		}
		if len(models) > 0 {
			return models[0], nil
		}
		return "", validationError("model", "学校尚未启用任何 AI 模型")
	}
	for _, model := range models {
		if model == requested {
			return requested, nil
		}
	}
	return "", validationError("model", "所选模型未启用，请联系管理员在学校设置中开启")
}

func (s *Service) saveAuditForSettings(ctx context.Context, principal auth.Principal, action string, detail map[string]any) {
	if s.repo == nil {
		return
	}
	_ = s.repo.CreateAudit(ctx, principal, AuditLog{
		Action:     action,
		TargetType: "school_ai_settings",
		TargetID:   principal.SchoolID.String(),
		Detail:     detail,
	})
}
