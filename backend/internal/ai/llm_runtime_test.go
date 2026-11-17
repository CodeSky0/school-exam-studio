package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestBreakerOpensAfterConsecutiveFailures(t *testing.T) {
	breaker := newBreakerSet(time.Minute, 3)
	key := "school|model"
	now := time.Now()
	if !breaker.allow(key, now) {
		t.Fatal("breaker should allow first request")
	}
	for index := 0; index < 3; index++ {
		breaker.failure(key, now)
	}
	if breaker.allow(key, now) {
		t.Fatal("breaker should be open after threshold")
	}
	if !breaker.allow(key, now.Add(2*time.Minute)) {
		t.Fatal("breaker should allow after cooldown")
	}
	breaker.success(key)
	if !breaker.allow(key, now) {
		t.Fatal("success should reset breaker")
	}
}

func TestOrderedModelsPrefersDefaultAndLimitsCandidates(t *testing.T) {
	ordered := orderedModels([]string{"a", "b", "c", "d"}, "b", "")
	if len(ordered) != 3 {
		t.Fatalf("expected 3 candidates, got %v", ordered)
	}
	if ordered[0] != "b" {
		t.Fatalf("expected default first, got %v", ordered)
	}
}

func TestCompleteWithCandidateRetriesRetryableErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(writer, `{"error":{"message":"temporary"}}`)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`)
	}))
	defer server.Close()

	service := &Service{breakers: newBreakerSet(time.Minute, 3), llmTimeout: 5 * time.Second}
	candidate := modelCandidate{
		Client: NewLLMClient(server.URL, "key", "model", 5*time.Second),
		Config: LLMConfig{Model: "model"},
		Key:    "school|model",
	}
	raw, err := service.completeWithCandidate(context.Background(), candidate, "system", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw != `{"ok":true}` {
		t.Fatalf("unexpected payload: %s", raw)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestCompleteWithCandidateRepairsInvalidJSON(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&calls, 1) == 1 {
			_, _ = io.WriteString(writer, `{"choices":[{"message":{"content":"这是解释而不是 JSON"}}]}`)
			return
		}
		_, _ = io.WriteString(writer, `{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`)
	}))
	defer server.Close()

	service := &Service{breakers: newBreakerSet(time.Minute, 3), llmTimeout: 5 * time.Second}
	candidate := modelCandidate{
		Client: NewLLMClient(server.URL, "key", "model", 5*time.Second),
		Config: LLMConfig{Model: "model"},
		Key:    "school|model",
	}
	raw, err := service.completeWithCandidate(context.Background(), candidate, "system", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw != `{"ok":true}` {
		t.Fatalf("unexpected payload: %s", raw)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected repair call, got %d calls", calls)
	}
}
