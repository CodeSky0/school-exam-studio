package drive

import (
	"strings"
)

const (
	chunkTargetRunes  = 700
	chunkOverlapRunes = 120
)

// splitContent 把长文本切成带重叠的段落，尽量在换行或句末断开。
// 中文按字符计数，避免按字节截断产生乱码。
func splitContent(text string) []string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return nil
	}
	runes := []rune(text)
	chunks := make([]string, 0, len(runes)/chunkTargetRunes+1)
	start := 0
	for start < len(runes) {
		end := start + chunkTargetRunes
		if end >= len(runes) {
			end = len(runes)
		} else {
			end = findChunkBoundary(runes, start+chunkTargetRunes/2, end)
		}
		if end <= start {
			end = start + chunkTargetRunes
			if end > len(runes) {
				end = len(runes)
			}
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(runes) {
			break
		}
		next := end - chunkOverlapRunes
		if next <= start {
			next = end
		}
		start = next
	}
	return chunks
}

// findChunkBoundary 在 [lower, upper) 内从后向前寻找自然断点，返回断点后的位置。
func findChunkBoundary(runes []rune, lower, upper int) int {
	if lower < 0 {
		lower = 0
	}
	if upper > len(runes) {
		upper = len(runes)
	}
	for index := upper - 1; index >= lower; index-- {
		if isChunkBoundary(runes[index]) {
			return index + 1
		}
	}
	return upper
}

func isChunkBoundary(value rune) bool {
	switch value {
	case '\n', '。', '！', '？', '；', '，', '、', '.', '!', '?', ';', ',', ' ':
		return true
	default:
		return false
	}
}
