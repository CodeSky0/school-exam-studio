package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	remoteBatchSize       = 32
	remoteMaxResponseSize = 16 << 20
	remoteProviderPrefix  = "remote:"
	maxProviderLength     = 80
)

// RemoteEmbedder 调用 OpenAI 兼容的 /embeddings 接口。
type RemoteEmbedder struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewRemote 创建远程向量器。baseURL 指向 OpenAI 兼容服务根地址，不含 /embeddings。
func NewRemote(baseURL, apiKey, model string) (*RemoteEmbedder, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("embedding base url must be an http(s) URL")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("embedding api key is required")
	}
	if strings.TrimSpace(model) == "" {
		model = "text-embedding-3-small"
	}
	return &RemoteEmbedder{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (e *RemoteEmbedder) Name() string {
	name := remoteProviderPrefix + e.model
	if len(name) > maxProviderLength {
		name = name[:maxProviderLength]
	}
	return name
}

// Dimensions 在首次调用前未知，返回 0 表示由服务端决定。
func (e *RemoteEmbedder) Dimensions() int { return 0 }

func (e *RemoteEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += remoteBatchSize {
		end := start + remoteBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]
		embedded, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, embedded...)
	}
	return vectors, nil
}

type remoteRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type remoteResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *RemoteEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	input := make([]string, len(texts))
	for index, text := range texts {
		input[index] = clampText(text)
	}
	payload, err := json.Marshal(remoteRequest{Model: e.model, Input: input})
	if err != nil {
		return nil, err
	}
	endpoint := e.baseURL + "/embeddings"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+e.apiKey)
	response, err := e.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, remoteMaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding service returned HTTP %d: %s", response.StatusCode, truncateMessage(string(body), 240))
	}
	var decoded remoteResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("embedding service returned %d vectors for %d inputs", len(decoded.Data), len(texts))
	}
	sort.Slice(decoded.Data, func(i, j int) bool { return decoded.Data[i].Index < decoded.Data[j].Index })
	vectors := make([][]float32, len(decoded.Data))
	dimensions := 0
	for index, item := range decoded.Data {
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("embedding service returned an empty vector")
		}
		if dimensions == 0 {
			dimensions = len(item.Embedding)
		} else if dimensions != len(item.Embedding) {
			return nil, fmt.Errorf("embedding service returned inconsistent dimensions")
		}
		Normalize(item.Embedding)
		vectors[index] = item.Embedding
	}
	return vectors, nil
}

func truncateMessage(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}
