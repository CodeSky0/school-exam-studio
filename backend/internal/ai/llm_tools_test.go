package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatToolsParsesToolCalls(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header missing")
		}
		if err := json.NewDecoder(request.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "先做题库体检",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {"name": "bank_audit", "arguments": "{\"limit\":500}"}
					}]
				},
				"finish_reason": "tool_calls"
			}]
		}`))
	}))
	defer server.Close()

	client := NewLLMClient(server.URL+"/v1", "test-key", "test-model", 5*time.Second)
	reply, err := client.ChatTools(context.Background(), []LLMMessage{{Role: "user", Content: "体检"}}, []LLMTool{{
		Type: "function",
		Function: LLMToolFunction{
			Name: "bank_audit", Description: "题库体检", Parameters: json.RawMessage(`{"type":"object"}`),
		},
	}})
	if err != nil {
		t.Fatalf("ChatTools: %v", err)
	}
	if reply.Content != "先做题库体检" || len(reply.ToolCalls) != 1 {
		t.Fatalf("reply = %#v", reply)
	}
	if reply.ToolCalls[0].Function.Name != "bank_audit" {
		t.Fatalf("tool name = %q", reply.ToolCalls[0].Function.Name)
	}
	if captured["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %#v", captured["tool_choice"])
	}
	if _, ok := captured["tools"]; !ok {
		t.Fatal("tools missing in request")
	}
}

func TestChatToolsReportsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":{"message":"rate limited","code":429}}`))
	}))
	defer server.Close()
	client := NewLLMClient(server.URL, "key", "model", 5*time.Second)
	_, err := client.ChatTools(context.Background(), []LLMMessage{{Role: "user", Content: "hi"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("error = %v", err)
	}
}

func TestWithModelArgumentOnlyInjectsForSupportedTools(t *testing.T) {
	args := map[string]any{"limit": 10}
	got := withModelArgument("search_questions", args, "gpt-4o-mini")
	if _, exists := got["model"]; exists {
		t.Fatal("search_questions should not receive model")
	}
	args = map[string]any{"question_id": "x"}
	got = withModelArgument("analyze_question", args, "gpt-4o-mini")
	if got["model"] != "gpt-4o-mini" {
		t.Fatalf("args = %#v", got)
	}
}
