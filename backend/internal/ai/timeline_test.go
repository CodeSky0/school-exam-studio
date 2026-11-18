package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"school-exam-studio/backend/internal/question"
)

func TestTimelineMapRoundTrip(t *testing.T) {
	entries := []TimelineEntry{
		{Type: TimelineTypeThought, Content: "先检索题库", CreatedAt: time.Now()},
		{Type: TimelineTypeTool, ToolCallID: "call_1", CreatedAt: time.Now()},
	}
	roundTrip := mapsToTimeline(timelineToMaps(entries))
	if len(roundTrip) != 2 || roundTrip[0].Type != TimelineTypeThought || roundTrip[1].ToolCallID != "call_1" {
		t.Fatalf("roundTrip = %#v", roundTrip)
	}
}

func TestRunLLMTurnInterleavesThoughtAndToolThenFinalReply(t *testing.T) {
	var round int32
	var firstPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		current := atomic.AddInt32(&round, 1)
		if current == 1 {
			_ = json.Unmarshal(body, &firstPayload)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sseFrame(`{"choices":[{"delta":{"content":"我先查看附件。"},"finish_reason":null}]}`))
			_, _ = io.WriteString(w, sseFrame(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"list_attachments","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`))
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sseFrame(`{"choices":[{"delta":{"content":"最终答复。"},"finish_reason":null}]}`))
		_, _ = io.WriteString(w, sseFrame(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`))
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	service := &Service{
		sanitizer: question.NewSanitizer(),
		breakers:  newBreakerSet(defaultBreakerOpen, breakerThreshold),
	}
	candidate := modelCandidate{
		Client: NewLLMClient(server.URL, "key", "model", 5*time.Second),
		Config: LLMConfig{Model: "model"},
		Key:    "school|model",
	}
	reply, calls, timeline, err := service.runLLMTurn(
		context.Background(), testPrincipal(), Conversation{ID: "6f1b0d7e-1f2a-4a4b-9c7d-3f0f3c9a1b2c"},
		nil, "测试", nil, "model", "run-1", candidate, nil,
	)
	if err != nil {
		t.Fatalf("runLLMTurn: %v", err)
	}
	if reply != "最终答复。" {
		t.Fatalf("reply = %q, want final answer only", reply)
	}
	if strings.Contains(reply, "我先查看附件") {
		t.Fatalf("reply should not contain intermediate narration: %q", reply)
	}
	if len(calls) != 1 || calls[0].Name != "list_attachments" {
		t.Fatalf("calls = %#v", calls)
	}
	if len(timeline) != 2 {
		t.Fatalf("timeline = %#v", timeline)
	}
	if timeline[0].Type != TimelineTypeThought || timeline[0].Content != "我先查看附件。" {
		t.Fatalf("first timeline entry = %#v", timeline[0])
	}
	if timeline[1].Type != TimelineTypeTool || timeline[1].ToolCallID != calls[0].ID {
		t.Fatalf("second timeline entry = %#v", timeline[1])
	}
	if _, exists := firstPayload["max_tokens"]; exists {
		t.Fatalf("request should not limit max_tokens: %#v", firstPayload)
	}
}

func sseFrame(payload string) string {
	return "data: " + payload + "\n\n"
}
