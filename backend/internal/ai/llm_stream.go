package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	maxLLMStreamBytes      = 8 << 20
	maxLLMStreamLineBytes  = 2 << 20
	maxLLMStreamErrorBytes = 64 << 10
)

// LLMDelta 是模型流式输出的最小增量；Content 与 Reasoning 至少一个非空。
type LLMDelta struct {
	Content   string
	Reasoning string
}

type streamText string

// UnmarshalJSON 兼容字符串与分段数组两种 content，并保留原始空白以便逐 Token 拼接。
func (s *streamText) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = streamText(text)
		return nil
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &parts); err != nil {
		return nil
	}
	builder := strings.Builder{}
	for _, part := range parts {
		builder.WriteString(part.Text)
	}
	*s = streamText(builder.String())
	return nil
}

type llmStreamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type llmStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          streamText          `json:"content"`
			Reasoning        streamText          `json:"reasoning"`
			ReasoningContent streamText          `json:"reasoning_content"`
			ToolCalls        []llmStreamToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// ChatToolsStream 调用 OpenAI 兼容的流式 function calling 接口。
// onDelta 会在每次收到正文或推理增量时同步回调；返回值为完整回复。
func (c *LLMClient) ChatToolsStream(ctx context.Context, messages []LLMMessage, tools []LLMTool, onDelta func(LLMDelta)) (LLMReply, error) {
	if !c.Enabled() {
		return LLMReply{}, fmt.Errorf("ai provider not configured")
	}
	payload, err := json.Marshal(llmToolRequest{
		Model:       c.model,
		Messages:    messages,
		Tools:       tools,
		ToolChoice:  "auto",
		Temperature: 0.2,
		MaxTokens:   4096,
		Stream:      true,
	})
	if err != nil {
		return LLMReply{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return LLMReply{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	// 流式请求不设置整体超时，由请求上下文负责取消；非流式仍沿用原超时。
	streamClient := &http.Client{Transport: c.http.Transport}
	response, err := streamClient.Do(request)
	if err != nil {
		return LLMReply{}, &ProviderError{Kind: ErrorKindRetryable, Message: fmt.Sprintf("模型接口请求失败：%v", err)}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxLLMStreamErrorBytes))
		var decoded llmToolResponse
		message := ""
		if json.Unmarshal(body, &decoded) == nil && decoded.Error != nil {
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
	reader := bufio.NewReader(response.Body)
	head, _ := reader.Peek(32)
	trimmed := bytes.TrimSpace(head)
	isEventStream := strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") ||
		bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:"))
	if !isEventStream {
		// 少数兼容网关忽略 stream 参数直接返回完整 JSON，此时退化为一次性输出。
		body, err := io.ReadAll(io.LimitReader(reader, maxLLMResponseBytes+1))
		if err != nil {
			return LLMReply{}, &ProviderError{Kind: ErrorKindRetryable, Message: "模型接口响应无法读取"}
		}
		if len(body) > maxLLMResponseBytes {
			return LLMReply{}, &ProviderError{Kind: ErrorKindFatal, Message: fmt.Sprintf("模型接口响应超过 %d 字节限制", maxLLMResponseBytes)}
		}
		var decoded llmToolResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
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
		reply := LLMReply{
			Content:      contentText(choice.Message.Content),
			ToolCalls:    choice.Message.ToolCalls,
			FinishReason: choice.FinishReason,
		}
		if onDelta != nil && reply.Content != "" {
			onDelta(LLMDelta{Content: reply.Content})
		}
		return reply, nil
	}
	return readLLMStream(reader, onDelta)
}

func readLLMStream(body io.Reader, onDelta func(LLMDelta)) (LLMReply, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLLMStreamLineBytes)
	content := strings.Builder{}
	reasoning := strings.Builder{}
	toolCalls := map[int]*LLMToolCall{}
	finishReason := ""
	total := 0
	data := strings.Builder{}
	done := false

	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		raw := strings.TrimSpace(data.String())
		data.Reset()
		if raw == "" {
			return nil
		}
		if raw == "[DONE]" {
			done = true
			return nil
		}
		var chunk llmStreamChunk
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			// 忽略厂商的注释或心跳帧，避免因非标准行中断整段回复。
			return nil
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return &ProviderError{Kind: ErrorKindRetryable, Message: sanitizeProviderMessage(chunk.Error.Message)}
		}
		for _, choice := range chunk.Choices {
			if value := strings.TrimSpace(choice.FinishReason); value != "" {
				finishReason = value
			}
			if text := string(choice.Delta.Content); text != "" {
				content.WriteString(text)
				if onDelta != nil {
					onDelta(LLMDelta{Content: text})
				}
			}
			text := string(choice.Delta.ReasoningContent)
			if text == "" {
				text = string(choice.Delta.Reasoning)
			}
			if text != "" {
				reasoning.WriteString(text)
				if onDelta != nil {
					onDelta(LLMDelta{Reasoning: text})
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				index := delta.Index
				entry, ok := toolCalls[index]
				if !ok {
					entry = &LLMToolCall{Type: "function"}
					toolCalls[index] = entry
				}
				if delta.ID != "" {
					entry.ID = delta.ID
				}
				if delta.Type != "" {
					entry.Type = delta.Type
				}
				entry.Function.Name += delta.Function.Name
				entry.Function.Arguments += delta.Function.Arguments
			}
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		total += len(line) + 1
		if total > maxLLMStreamBytes {
			return buildStreamReply(content, reasoning, toolCalls, finishReason), &ProviderError{
				Kind: ErrorKindFatal, Message: fmt.Sprintf("模型流式响应超过 %d 字节限制", maxLLMStreamBytes),
			}
		}
		line = strings.TrimRight(line, "\r")
		if line == "" {
			if err := flush(); err != nil {
				return buildStreamReply(content, reasoning, toolCalls, finishReason), err
			}
			if done {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteString("\n")
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil && !done {
		return buildStreamReply(content, reasoning, toolCalls, finishReason), &ProviderError{Kind: ErrorKindRetryable, Message: "模型流式响应中断"}
	}
	if !done {
		if err := flush(); err != nil {
			return buildStreamReply(content, reasoning, toolCalls, finishReason), err
		}
	}
	reply := buildStreamReply(content, reasoning, toolCalls, finishReason)
	if strings.TrimSpace(reply.Content) == "" && len(reply.ToolCalls) == 0 {
		return reply, &ProviderError{
			Kind: ErrorKindRetryable, Message: fmt.Sprintf("模型接口返回空内容（finish_reason=%s）", strings.TrimSpace(finishReason)),
		}
	}
	return reply, nil
}

func buildStreamReply(content, reasoning strings.Builder, toolCalls map[int]*LLMToolCall, finishReason string) LLMReply {
	indexes := make([]int, 0, len(toolCalls))
	for index := range toolCalls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	calls := make([]LLMToolCall, 0, len(indexes))
	for position, index := range indexes {
		call := *toolCalls[index]
		if call.ID == "" {
			call.ID = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), position)
		}
		if call.Type == "" {
			call.Type = "function"
		}
		calls = append(calls, call)
	}
	return LLMReply{
		Content:      strings.TrimSpace(content.String()),
		Reasoning:    strings.TrimSpace(reasoning.String()),
		ToolCalls:    calls,
		FinishReason: finishReason,
	}
}

// chatToolsStream 与 chatTools 使用相同的重试策略；一旦已经向用户输出 Token，
// 不再重试，避免同一段内容重复展示。
func (s *Service) chatToolsStream(ctx context.Context, candidate modelCandidate, messages []LLMMessage, tools []LLMTool, onDelta func(LLMDelta)) (LLMReply, error) {
	var lastErr error
	emitted := false
	wrapped := func(delta LLMDelta) {
		if delta.Content != "" || delta.Reasoning != "" {
			emitted = true
		}
		if onDelta != nil {
			onDelta(delta)
		}
	}
	for attempt := 1; attempt <= modelMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return LLMReply{}, err
		}
		reply, err := candidate.Client.ChatToolsStream(ctx, messages, tools, wrapped)
		if err == nil {
			s.breaker().success(candidate.Key)
			return reply, nil
		}
		lastErr = err
		if emitted {
			s.breaker().failure(candidate.Key, time.Now())
			return reply, err
		}
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
