package ai

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadLLMStreamAggregatesTokensAndToolCalls(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"先查一下"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"你"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"好"}}]}`,
		``,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"search_","arguments":"{\"key"}}]}}]}`,
		``,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"questions","arguments":"word\":\"函数\"}"}}]},"finish_reason":"tool_calls"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	deltas := []LLMDelta{}
	reply, err := readLLMStream(strings.NewReader(stream), func(delta LLMDelta) {
		deltas = append(deltas, delta)
	})
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if reply.Content != "你好" {
		t.Fatalf("content = %q", reply.Content)
	}
	if reply.Reasoning != "先查一下" {
		t.Fatalf("reasoning = %q", reply.Reasoning)
	}
	if reply.FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %q", reply.FinishReason)
	}
	if len(reply.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(reply.ToolCalls))
	}
	call := reply.ToolCalls[0]
	if call.Function.Name != "search_questions" {
		t.Fatalf("tool name = %q", call.Function.Name)
	}
	if call.Function.Arguments != `{"keyword":"函数"}` {
		t.Fatalf("tool arguments = %q", call.Function.Arguments)
	}
	if len(deltas) != 3 || deltas[0].Reasoning != "先查一下" || deltas[1].Content != "你" || deltas[2].Content != "好" {
		t.Fatalf("deltas = %#v", deltas)
	}
}

func TestReadLLMStreamErrorsOnEmptyReply(t *testing.T) {
	_, err := readLLMStream(strings.NewReader("data: [DONE]\n\n"), nil)
	if err == nil {
		t.Fatal("expected empty reply error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != ErrorKindRetryable {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatToolsStreamSendsStreamFlagAndParsesResponse(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &payload)
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)
		flusher, _ := writer.(http.Flusher)
		_, _ = io.WriteString(writer, "data: {\"choices\":[{\"delta\":{\"content\":\"流式\"}}]}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(writer, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewLLMClient(server.URL, "key", "model", 5*time.Second)
	reply, err := client.ChatToolsStream(t.Context(), []LLMMessage{{Role: "user", Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if reply.Content != "流式" {
		t.Fatalf("content = %q", reply.Content)
	}
	if payload["stream"] != true {
		t.Fatalf("stream flag = %#v", payload["stream"])
	}
}

func TestChatToolsStreamFallsBackToJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"choices":[{"message":{"role":"assistant","content":"一次性回复"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	client := NewLLMClient(server.URL, "key", "model", 5*time.Second)
	deltas := []LLMDelta{}
	reply, err := client.ChatToolsStream(t.Context(), []LLMMessage{{Role: "user", Content: "hi"}}, nil, func(delta LLMDelta) {
		deltas = append(deltas, delta)
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if reply.Content != "一次性回复" || len(deltas) != 1 || deltas[0].Content != "一次性回复" {
		t.Fatalf("reply = %#v, deltas = %#v", reply, deltas)
	}
}

func TestChatToolsStreamMapsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(writer, `{"error":{"message":"rate limited","code":429}}`)
	}))
	defer server.Close()

	client := NewLLMClient(server.URL, "key", "model", 5*time.Second)
	_, err := client.ChatToolsStream(t.Context(), []LLMMessage{{Role: "user", Content: "hi"}}, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if providerErr.StatusCode != http.StatusTooManyRequests || providerErr.Message != "rate limited" {
		t.Fatalf("provider error = %#v", providerErr)
	}
}
