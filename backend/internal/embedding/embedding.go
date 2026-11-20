// Package embedding 为云盘资料与 AI 检索提供文本向量化能力。
//
// 未配置远程模型时使用确定性的本地哈希向量，保证上传资料在离线环境下
// 也能被检索；配置 OpenAI 兼容的 /embeddings 接口后可切换为语义向量。
package embedding

import (
	"context"
	"math"
	"strings"
)

// Embedder 把一批文本转换为单位向量。
type Embedder interface {
	Name() string
	Dimensions() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

const (
	// LocalName 是内置确定性向量的提供方标识，写入索引后用于区分向量来源。
	LocalName = "local-hash-v1"
	// MaxTextRunes 限制单条文本长度，避免异常输入产生超大请求。
	MaxTextRunes = 4000
)

// New 根据配置选择向量化实现：远程配置完整时使用 OpenAI 兼容接口，
// 否则回退到本地哈希向量。接口地址必须是 http/https。
func New(baseURL, apiKey, model string, localDimensions int) (Embedder, error) {
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if baseURL != "" && apiKey != "" {
		return NewRemote(baseURL, apiKey, model)
	}
	return NewLocal(localDimensions), nil
}

// Cosine 计算两个向量的余弦相似度，维度不一致时返回 0。
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for index := range a {
		dot += float64(a[index]) * float64(b[index])
		normA += float64(a[index]) * float64(a[index])
		normB += float64(b[index]) * float64(b[index])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / math.Sqrt(normA*normB)
}

// Normalize 原地把向量缩放为单位长度，全零向量保持原样。
func Normalize(values []float32) {
	var sum float64
	for _, value := range values {
		sum += float64(value) * float64(value)
	}
	if sum == 0 {
		return
	}
	scale := 1 / math.Sqrt(sum)
	for index := range values {
		values[index] = float32(float64(values[index]) * scale)
	}
}

// clampText 按字符截断文本，避免单个分段超出模型输入限制。
func clampText(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= MaxTextRunes {
		return string(runes)
	}
	return string(runes[:MaxTextRunes])
}
