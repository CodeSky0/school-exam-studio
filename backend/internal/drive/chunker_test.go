package drive

import (
	"strings"
	"testing"
)

func TestSplitContentKeepsShortTextTogether(t *testing.T) {
	chunks := splitContent("第一行\n\n第二行")
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "第一行") || !strings.Contains(chunks[0], "第二行") {
		t.Fatalf("unexpected chunk content: %q", chunks[0])
	}
}

func TestSplitContentEmpty(t *testing.T) {
	if chunks := splitContent("   \n\n "); len(chunks) != 0 {
		t.Fatalf("expected no chunks, got %d", len(chunks))
	}
}

func TestSplitContentProducesBoundedOverlappingChunks(t *testing.T) {
	sentence := "函数与几何是初中数学的重要内容。"
	text := strings.Repeat(sentence, 120)
	chunks := splitContent(text)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for index, chunk := range chunks {
		length := len([]rune(chunk))
		if length > chunkTargetRunes {
			t.Fatalf("chunk %d has %d runes, exceeds target %d", index, length, chunkTargetRunes)
		}
	}
	if !strings.Contains(chunks[0], sentence[:6]) {
		t.Fatalf("first chunk does not start with expected content: %q", chunks[0])
	}
	last := len(chunks) - 1
	if !strings.HasSuffix(chunks[last], sentence) {
		t.Fatalf("last chunk does not end with source text: %q", chunks[last])
	}
}

func TestSplitContentOverlapsAdjacentChunks(t *testing.T) {
	text := strings.Repeat("这是一个用于测试重叠的句子。", 80)
	chunks := splitContent(text)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	first := []rune(chunks[0])
	tail := string(first[len(first)-chunkOverlapRunes/2:])
	if !strings.Contains(chunks[1], tail) {
		t.Fatalf("expected second chunk to overlap the first")
	}
}
