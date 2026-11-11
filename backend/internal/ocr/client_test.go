package ocr_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"school-exam-studio/backend/internal/ocr"
)

func TestClientForwardsMultipartAndDecodesText(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/recognize" || r.Method != http.MethodPost {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Errorf("multipart request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "image" || part.FileName() != "question.png" || part.Header.Get("Content-Type") != "image/png" {
			t.Errorf("unexpected image part: %+v %v", part, err)
		}
		data, err := io.ReadAll(part)
		if err != nil || len(data) == 0 {
			t.Errorf("image body was not forwarded: %d %v", len(data), err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"已知 $x^2=4$"}`)
	}))
	defer upstream.Close()
	client, err := ocr.NewClient(upstream.URL, time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	text, err := client.Recognize(context.Background(), testPNG(t), "question.png", "image/png")
	if err != nil || text != "已知 $x^2=4$" {
		t.Fatalf("recognize text=%q err=%v", text, err)
	}
}

func TestClientMapsUpstreamFailures(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		expected error
	}{
		{"not ready", http.StatusServiceUnavailable, `{}`, ocr.ErrUnavailable},
		{"upstream failure", http.StatusInternalServerError, `{}`, ocr.ErrUpstream},
		{"invalid response", http.StatusOK, `not-json`, ocr.ErrUpstream},
		{"empty result", http.StatusOK, `{"text":"  "}`, ocr.ErrEmptyResult},
		{"sidecar empty result", http.StatusUnprocessableEntity, `{"error":{"code":"EMPTY_RECOGNITION"}}`, ocr.ErrEmptyResult},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer upstream.Close()
			client, err := ocr.NewClient(upstream.URL, time.Second, 1)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Recognize(context.Background(), testPNG(t), "question.png", "image/png")
			if !errors.Is(err, test.expected) {
				t.Fatalf("error=%v, want %v", err, test.expected)
			}
		})
	}
}

func TestClientTimeoutUnavailableAndResponseLimit(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(150 * time.Millisecond)
			_, _ = io.WriteString(w, `{"text":"late"}`)
		}))
		defer upstream.Close()
		client, err := ocr.NewClient(upstream.URL, 20*time.Millisecond, 1)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Recognize(context.Background(), testPNG(t), "question.png", "image/png")
		if !errors.Is(err, ocr.ErrTimeout) {
			t.Fatalf("expected timeout, got %v", err)
		}
	})
	t.Run("unavailable", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		client, err := ocr.NewClient(upstream.URL, 100*time.Millisecond, 1)
		if err != nil {
			t.Fatal(err)
		}
		upstream.Close()
		_, err = client.Recognize(context.Background(), testPNG(t), "question.png", "image/png")
		if !errors.Is(err, ocr.ErrUnavailable) {
			t.Fatalf("expected unavailable, got %v", err)
		}
	})
	t.Run("response limit", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"text":"`+strings.Repeat("x", int(ocr.DefaultMaxResponseSize)+1)+`"}`)
		}))
		defer upstream.Close()
		client, err := ocr.NewClient(upstream.URL, time.Second, 1)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Recognize(context.Background(), testPNG(t), "question.png", "image/png")
		if !errors.Is(err, ocr.ErrUpstream) {
			t.Fatalf("expected response-size failure, got %v", err)
		}
	})
}

func TestClientBoundsConcurrency(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, `{"text":"ok"}`)
	}))
	defer upstream.Close()
	client, err := ocr.NewClient(upstream.URL, 2*time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	image := testPNG(t)
	var wait sync.WaitGroup
	errorsCh := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := client.Recognize(context.Background(), image, "question.png", "image/png")
			errorsCh <- err
		}()
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := maximum.Load(); got > 2 {
		t.Fatalf("maximum upstream concurrency=%d, want <=2", got)
	}
}
