package embedding

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalEmbedderProducesNormalizedVectors(t *testing.T) {
	embedder := NewLocal(128)
	vectors, err := embedder.Embed(context.Background(), []string{"一元一次方程", "函数与几何", ""})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vectors) != 3 {
		t.Fatalf("expected 3 vectors, got %d", len(vectors))
	}
	for index, vector := range vectors {
		if len(vector) != 128 {
			t.Fatalf("vector %d has %d dimensions", index, len(vector))
		}
		if index < 2 && mathNorm(vector) < 0.999 {
			t.Fatalf("vector %d is not normalized", index)
		}
	}
}

func TestLocalEmbedderRanksRelatedTextHigher(t *testing.T) {
	embedder := NewLocal(256)
	vectors, err := embedder.Embed(context.Background(), []string{
		"一元一次方程的解法与练习题",
		"二次函数图像与性质教案",
		"如何求解一元一次方程",
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	related := Cosine(vectors[0], vectors[2])
	unrelated := Cosine(vectors[0], vectors[1])
	if related <= unrelated {
		t.Fatalf("expected related text to score higher: related=%.3f unrelated=%.3f", related, unrelated)
	}
}

func TestCosineHandlesInvalidInput(t *testing.T) {
	if score := Cosine([]float32{1, 0}, []float32{1}); score != 0 {
		t.Fatalf("expected 0 for mismatched dimensions, got %f", score)
	}
	if score := Cosine([]float32{0, 0}, []float32{1, 1}); score != 0 {
		t.Fatalf("expected 0 for zero vector, got %f", score)
	}
	if score := Cosine([]float32{1, 0}, []float32{1, 0}); score < 0.999 {
		t.Fatalf("expected identical vectors to score 1, got %f", score)
	}
}

func TestNewFallsBackToLocal(t *testing.T) {
	embedder, err := New("", "", "", 0)
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	if embedder.Name() != LocalName {
		t.Fatalf("expected local embedder, got %s", embedder.Name())
	}
}

func TestRemoteEmbedderCallsOpenAICompatibleEndpoint(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("unexpected authorization: %s", r.Header.Get("Authorization"))
		}
		var request remoteRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if request.Model != "text-embedding-test" {
			t.Errorf("unexpected model: %s", request.Model)
		}
		data := make([]map[string]any, 0, len(request.Input))
		for index := len(request.Input) - 1; index >= 0; index-- {
			data = append(data, map[string]any{
				"index":     index,
				"embedding": []float32{float32(index + 1), 1},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()

	embedder, err := NewRemote(server.URL+"/v1", "secret-key", "text-embedding-test")
	if err != nil {
		t.Fatalf("new remote embedder: %v", err)
	}
	vectors, err := embedder.Embed(context.Background(), []string{"甲", "乙"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/v1/embeddings" {
		t.Fatalf("unexpected request paths: %v", paths)
	}
	if len(vectors) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vectors))
	}
	if vectors[0][0] >= vectors[1][0] {
		t.Fatalf("expected vectors to be restored by index order, got %v", vectors)
	}
	for index, vector := range vectors {
		if normalized := mathNorm(vector); normalized < 0.999 || normalized > 1.001 {
			t.Fatalf("vector %d is not normalized: %f", index, normalized)
		}
	}
}

func TestRemoteEmbedderRejectsPartialResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"index": 0, "embedding": []float32{1}}}})
	}))
	defer server.Close()
	embedder, err := NewRemote(server.URL, "key", "model")
	if err != nil {
		t.Fatalf("new remote embedder: %v", err)
	}
	if _, err := embedder.Embed(context.Background(), []string{"甲", "乙"}); err == nil {
		t.Fatal("expected partial response to fail")
	}
}

func TestNewRemoteValidatesBaseURL(t *testing.T) {
	if _, err := NewRemote("ftp://example.com", "key", "model"); err == nil {
		t.Fatal("expected invalid scheme to fail")
	}
	if _, err := NewRemote("https://example.com", "", "model"); err == nil {
		t.Fatal("expected missing api key to fail")
	}
}

func mathNorm(vector []float32) float64 {
	var sum float64
	for _, value := range vector {
		sum += float64(value) * float64(value)
	}
	return math.Sqrt(sum)
}
