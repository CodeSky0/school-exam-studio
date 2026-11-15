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

const maxLLMResponseBytes = 2 << 20

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
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *LLMClient) CompleteJSON(ctx context.Context, system, user string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("ai provider not configured")
	}
	payload, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
		MaxTokens:   2400,
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
		return "", fmt.Errorf("ai provider request failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLLMResponseBytes))
	if err != nil {
		return "", fmt.Errorf("ai provider response unreadable")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("ai provider returned status %d", response.StatusCode)
	}
	var decoded chatResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("ai provider returned invalid payload")
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", fmt.Errorf("ai provider rejected the request")
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("ai provider returned no choices")
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("ai provider returned empty content")
	}
	return extractJSON(content), nil
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
			raw = raw[newline+1:]
		}
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	}
	start := -1
	for index, char := range raw {
		if char == '{' || char == '[' {
			start = index
			break
		}
	}
	if start < 0 {
		return raw
	}
	end := -1
	for index := len(raw) - 1; index >= start; index-- {
		if raw[index] == '}' || raw[index] == ']' {
			end = index
			break
		}
	}
	if end < start {
		return raw
	}
	return strings.TrimSpace(raw[start : end+1])
}
