package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 输出长度不设业务上限；这里仅保留防止异常响应耗尽内存的硬保护。
const maxLLMResponseBytes = 64 << 20

type LLMClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewLLMClient(baseURL, apiKey, model string, timeout time.Duration) *LLMClient {
	if timeout < 5*time.Second {
		timeout = 45 * time.Second
	}
	return &LLMClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *LLMClient) Enabled() bool {
	return c != nil && c.baseURL != "" && c.apiKey != "" && c.model != ""
}

func (c *LLMClient) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

type chatMessage struct {
	Role             string          `json:"role"`
	Content          json.RawMessage `json:"content"`
	Reasoning        string          `json:"reasoning"`
	ReasoningContent string          `json:"reasoning_content"`
}

// text 兼容字符串与分片数组两种 content 形式，并在仅有推理内容时回退。
func (m chatMessage) text() string {
	if value := contentText(m.Content); value != "" {
		return value
	}
	return strings.TrimSpace(firstNonEmpty(m.ReasoningContent, m.Reasoning))
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	builder := strings.Builder{}
	for _, part := range parts {
		if value := strings.TrimSpace(part.Text); value != "" {
			builder.WriteString(value)
		}
	}
	return strings.TrimSpace(builder.String())
}

// ProviderError 描述模型服务返回的业务错误，保留状态码、错误类别与厂商消息便于弹性层决策。
type ProviderError struct {
	StatusCode int
	Message    string
	Kind       ErrorKind
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	if e.StatusCode == 0 {
		if e.Message != "" {
			return e.Message
		}
		return "模型接口请求失败"
	}
	if e.Message != "" {
		return fmt.Sprintf("模型接口返回 HTTP %d：%s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("模型接口返回 HTTP %d", e.StatusCode)
}

type outgoingMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string            `json:"model"`
	Messages    []outgoingMessage `json:"messages"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (c *LLMClient) CompleteJSON(ctx context.Context, system, user string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("ai provider not configured")
	}
	payload, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []outgoingMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	response, err := c.http.Do(request)
	if err != nil {
		return "", &ProviderError{Kind: ErrorKindRetryable, Message: fmt.Sprintf("模型接口请求失败：%v", err)}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLLMResponseBytes+1))
	if err != nil {
		return "", &ProviderError{Kind: ErrorKindRetryable, Message: "模型接口响应无法读取"}
	}
	if len(body) > maxLLMResponseBytes {
		return "", &ProviderError{Kind: ErrorKindFatal, Message: fmt.Sprintf("模型接口响应超过 %d 字节限制", maxLLMResponseBytes)}
	}
	var decoded chatResponse
	decodeErr := json.Unmarshal(body, &decoded)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := ""
		if decodeErr == nil && decoded.Error != nil {
			message = decoded.Error.Message
		}
		if message == "" {
			message = providerMessage(body)
		}
		return "", &ProviderError{
			StatusCode: response.StatusCode,
			Message:    sanitizeProviderMessage(message),
			Kind:       classifyStatus(response.StatusCode),
			RetryAfter: retryAfterFromHeader(response.Header),
		}
	}
	if decodeErr != nil {
		return "", &ProviderError{Kind: ErrorKindParse, Message: "模型接口返回了无法解析的响应"}
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", &ProviderError{
			StatusCode: response.StatusCode,
			Message:    sanitizeProviderMessage(decoded.Error.Message),
			Kind:       classifyStatus(response.StatusCode),
			RetryAfter: retryAfterFromHeader(response.Header),
		}
	}
	if len(decoded.Choices) == 0 {
		return "", &ProviderError{Kind: ErrorKindRetryable, Message: "模型接口未返回任何结果"}
	}
	content := decoded.Choices[0].Message.text()
	if content == "" {
		if reason := strings.TrimSpace(decoded.Choices[0].FinishReason); reason != "" {
			return "", &ProviderError{Kind: ErrorKindRetryable, Message: fmt.Sprintf("模型接口返回空内容（finish_reason=%s）", reason)}
		}
		return "", &ProviderError{Kind: ErrorKindRetryable, Message: "模型接口返回空内容"}
	}
	// 输出达到 token 上限时 JSON 会被截断，直接返回内容只会得到含糊的解析失败，需给出可操作的错误。
	if strings.EqualFold(strings.TrimSpace(decoded.Choices[0].FinishReason), "length") {
		return "", &ProviderError{Kind: ErrorKindFatal, Message: "模型输出被截断（finish_reason=length），请减少生成数量或提高模型输出上限"}
	}
	return extractJSON(content), nil
}

func providerMessage(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if payload.Error.Message != "" {
			return payload.Error.Message
		}
		return payload.Message
	}
	return strings.TrimSpace(string(body))
}

func sanitizeProviderMessage(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	if message == "" {
		return ""
	}
	runes := []rune(message)
	if len(runes) > 300 {
		message = string(runes[:300]) + "…"
	}
	return message
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
			raw = raw[newline+1:]
		}
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	}
	// 优先定位 JSON 对象：模型常在 JSON 之前输出带方括号的说明文字，若先匹配 '[' 会截错范围。
	start := strings.IndexByte(raw, '{')
	closing := byte('}')
	if start < 0 {
		start = strings.IndexByte(raw, '[')
		closing = ']'
	}
	if start < 0 {
		return raw
	}
	end := strings.LastIndexByte(raw, closing)
	if end < start {
		return raw
	}
	return strings.TrimSpace(raw[start : end+1])
}
