package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LLMToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type LLMTool struct {
	Type     string          `json:"type"`
	Function LLMToolFunction `json:"function"`
}

type LLMToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// LLMImageURL 是多模态消息中的图片地址，支持 data: URL 直接内联原始图片。
type LLMImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// LLMContentPart 是 OpenAI 兼容的多模态内容片段：text 或 image_url。
type LLMContentPart struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *LLMImageURL `json:"image_url,omitempty"`
}

type LLMMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	Parts      []LLMContentPart `json:"-"`
	ToolCalls  []LLMToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

// MarshalJSON 在没有多模态片段时保持 content 为字符串，有图片时输出分段数组。
func (m LLMMessage) MarshalJSON() ([]byte, error) {
	type messagePayload struct {
		Role       string        `json:"role"`
		Content    any           `json:"content"`
		ToolCalls  []LLMToolCall `json:"tool_calls,omitempty"`
		ToolCallID string        `json:"tool_call_id,omitempty"`
		Name       string        `json:"name,omitempty"`
	}
	payload := messagePayload{
		Role:       m.Role,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
		Name:       m.Name,
	}
	if len(m.Parts) > 0 {
		payload.Content = m.Parts
	} else {
		payload.Content = m.Content
	}
	return json.Marshal(payload)
}

type LLMReply struct {
	Content      string
	Reasoning    string
	ToolCalls    []LLMToolCall
	FinishReason string
}

type llmToolRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Tools       []LLMTool    `json:"tools,omitempty"`
	ToolChoice  string       `json:"tool_choice,omitempty"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
}

type llmToolResponse struct {
	Choices []struct {
		Message struct {
			Content   json.RawMessage `json:"content"`
			ToolCalls []LLMToolCall   `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// ChatTools 调用 OpenAI 兼容的 function calling 接口，返回自然语言与工具调用。
func (c *LLMClient) ChatTools(ctx context.Context, messages []LLMMessage, tools []LLMTool) (LLMReply, error) {
	if !c.Enabled() {
		return LLMReply{}, fmt.Errorf("ai provider not configured")
	}
	payload, err := json.Marshal(llmToolRequest{
		Model:       c.model,
		Messages:    messages,
		Tools:       tools,
		ToolChoice:  "auto",
		Temperature: 0.2,
	})
	if err != nil {
		return LLMReply{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return LLMReply{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	response, err := c.http.Do(request)
	if err != nil {
		return LLMReply{}, &ProviderError{Kind: ErrorKindRetryable, Message: fmt.Sprintf("模型接口请求失败：%v", err)}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLLMResponseBytes+1))
	if err != nil {
		return LLMReply{}, &ProviderError{Kind: ErrorKindRetryable, Message: "模型接口响应无法读取"}
	}
	if len(body) > maxLLMResponseBytes {
		return LLMReply{}, &ProviderError{Kind: ErrorKindFatal, Message: fmt.Sprintf("模型接口响应超过 %d 字节限制", maxLLMResponseBytes)}
	}
	var decoded llmToolResponse
	decodeErr := json.Unmarshal(body, &decoded)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := ""
		if decodeErr == nil && decoded.Error != nil {
			message = decoded.Error.Message
		}
		if message == "" {
			message = providerMessage(body)
		}
		return LLMReply{}, &ProviderError{
			StatusCode: response.StatusCode,
			Message:    sanitizeProviderMessage(message),
			Kind:       classifyStatus(response.StatusCode),
			RetryAfter: retryAfterFromHeader(response.Header),
		}
	}
	if decodeErr != nil {
		return LLMReply{}, &ProviderError{Kind: ErrorKindParse, Message: "模型接口返回了无法解析的响应"}
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return LLMReply{}, &ProviderError{
			StatusCode: response.StatusCode,
			Message:    sanitizeProviderMessage(decoded.Error.Message),
			Kind:       classifyStatus(response.StatusCode),
			RetryAfter: retryAfterFromHeader(response.Header),
		}
	}
	if len(decoded.Choices) == 0 {
		return LLMReply{}, &ProviderError{Kind: ErrorKindRetryable, Message: "模型接口未返回任何结果"}
	}
	choice := decoded.Choices[0]
	content := contentText(choice.Message.Content)
	if content == "" && len(choice.Message.ToolCalls) == 0 {
		return LLMReply{}, &ProviderError{Kind: ErrorKindRetryable, Message: fmt.Sprintf("模型接口返回空内容（finish_reason=%s）", strings.TrimSpace(choice.FinishReason))}
	}
	return LLMReply{
		Content:      content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
	}, nil
}
