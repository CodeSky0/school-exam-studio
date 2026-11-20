package embedding

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// LocalHashEmbedder 使用字符与二元组特征哈希生成确定性向量。
// 它不依赖外部服务，适合开发环境、离线部署与远程模型不可用时的兜底。
type LocalHashEmbedder struct {
	dimensions int
}

// NewLocal 创建一个本地哈希向量器；维度会被限制在 64-1024 之间。
func NewLocal(dimensions int) *LocalHashEmbedder {
	if dimensions < 64 || dimensions > 1024 {
		dimensions = 256
	}
	return &LocalHashEmbedder{dimensions: dimensions}
}

func (e *LocalHashEmbedder) Name() string { return LocalName }

func (e *LocalHashEmbedder) Dimensions() int { return e.dimensions }

func (e *LocalHashEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for index, text := range texts {
		vectors[index] = e.vectorize(text)
	}
	return vectors, nil
}

// vectorize 将文本拆成词、单字与相邻二元组后做特征哈希。
// 中文单字与二元组能覆盖常见术语，英文与数字按连续片段处理。
func (e *LocalHashEmbedder) vectorize(text string) []float32 {
	vector := make([]float32, e.dimensions)
	text = strings.ToLower(clampText(text))
	tokens := localTokens(text)
	if len(tokens) == 0 {
		return vector
	}
	weights := make(map[string]float64, len(tokens))
	for _, token := range tokens {
		weights[token] += 1
	}
	for token, count := range weights {
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(token))
		sum := hasher.Sum32()
		position := int(sum % uint32(e.dimensions))
		sign := 1.0
		if sum&(1<<31) != 0 {
			sign = -1
		}
		// 对数抑制高频虚词的影响，保留关键词区分度。
		vector[position] += float32(sign * (1 + math.Log(count)))
	}
	Normalize(vector)
	return vector
}

// localTokens 返回英文数字片段、中文单字与中文二元组。
func localTokens(text string) []string {
	runes := []rune(text)
	tokens := make([]string, 0, len(runes)*2)
	var word []rune
	flushWord := func() {
		if len(word) > 0 {
			tokens = append(tokens, string(word))
			word = word[:0]
		}
	}
	var previous rune
	hasPrevious := false
	for _, current := range runes {
		if unicode.Is(unicode.Han, current) {
			flushWord()
			tokens = append(tokens, string(current))
			if hasPrevious && unicode.Is(unicode.Han, previous) {
				tokens = append(tokens, string(previous)+string(current))
			}
			previous, hasPrevious = current, true
			continue
		}
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			word = append(word, current)
			previous, hasPrevious = current, true
			continue
		}
		flushWord()
		previous, hasPrevious = 0, false
	}
	flushWord()
	return tokens
}
