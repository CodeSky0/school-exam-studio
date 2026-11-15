package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCompleteJSONReadsContentShapes(t *testing.T) {
	cases := map[string]string{
		"string":            `{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`,
		"content_parts":     `{"choices":[{"message":{"content":[{"type":"text","text":"{\"ok\":"},{"type":"text","text":"true}"}]}}]}`,
		"reasoning":         `{"choices":[{"message":{"content":null,"reasoning":"{\"ok\":true}"}}]}`,
		"reasoning_content": `{"choices":[{"message":{"content":"","reasoning_content":"{\"ok\":true}"}}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()

			client := NewLLMClient(server.URL, "test-key", "test-model", 5*time.Second)
			raw, err := client.CompleteJSON(context.Background(), "system", "user")
			if err != nil {
				t.Fatalf("CompleteJSON: %v", err)
			}
			if strings.TrimSpace(raw) != `{"ok":true}` {
				t.Fatalf("unexpected payload: %s", raw)
			}
		})
	}
}

func TestCompleteJSONSurfacesProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"message":"Thinking Machines is only available on agentic harnesses"}}`)
	}))
	defer server.Close()

	client := NewLLMClient(server.URL, "test-key", "test-model", 5*time.Second)
	_, err := client.CompleteJSON(context.Background(), "system", "user")
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %v", err)
	}
	if providerErr.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", providerErr.StatusCode)
	}
	if !strings.Contains(providerErr.Error(), "agentic harnesses") {
		t.Fatalf("provider message lost: %s", providerErr.Error())
	}
}

func TestCompleteJSONReportsEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`)
	}))
	defer server.Close()

	client := NewLLMClient(server.URL, "test-key", "test-model", 5*time.Second)
	_, err := client.CompleteJSON(context.Background(), "system", "user")
	if err == nil || !strings.Contains(err.Error(), "finish_reason=length") {
		t.Fatalf("unexpected error: %v", err)
	}
}
