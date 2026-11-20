package drive

import (
	"strings"
	"testing"
)

func TestRankMatchesLimitsHitsPerFileAndScoreFloor(t *testing.T) {
	matches := []ChunkMatch{
		{ItemID: "a", ItemName: "教案.pdf", ChunkIndex: 0, Score: 0.9, Content: "函数图像"},
		{ItemID: "a", ItemName: "教案.pdf", ChunkIndex: 1, Score: 0.8, Content: "一次函数"},
		{ItemID: "a", ItemName: "教案.pdf", ChunkIndex: 2, Score: 0.7, Content: "二次函数"},
		{ItemID: "b", ItemName: "试卷.docx", ChunkIndex: 0, Score: 0.6, Content: "方程练习"},
		{ItemID: "c", ItemName: "无关.txt", ChunkIndex: 0, Score: 0.01, Content: "天气"},
	}
	hits := rankMatches(matches, 10)
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d: %+v", len(hits), hits)
	}
	if hits[0].ItemID != "a" || hits[0].ChunkIndex != 0 {
		t.Fatalf("unexpected first hit: %+v", hits[0])
	}
	if hits[1].ItemID != "a" || hits[1].ChunkIndex != 1 {
		t.Fatalf("unexpected second hit: %+v", hits[1])
	}
	if hits[2].ItemID != "b" {
		t.Fatalf("expected file b after per-file cap, got %+v", hits[2])
	}
}

func TestRankMatchesRespectsLimit(t *testing.T) {
	matches := []ChunkMatch{
		{ItemID: "a", Score: 0.9, Content: "一"},
		{ItemID: "b", Score: 0.8, Content: "二"},
		{ItemID: "c", Score: 0.7, Content: "三"},
	}
	hits := rankMatches(matches, 2)
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
}

func TestExcerptTextTruncatesByRunes(t *testing.T) {
	value := strings.Repeat("数学", 200)
	excerpt := excerptText(value, 20)
	if len([]rune(excerpt)) != 21 {
		t.Fatalf("expected 20 runes plus ellipsis, got %d", len([]rune(excerpt)))
	}
	if !strings.HasSuffix(excerpt, "…") {
		t.Fatalf("expected ellipsis suffix: %q", excerpt)
	}
}

func TestRoundScore(t *testing.T) {
	if score := roundScore(0.123456); score != 0.1235 {
		t.Fatalf("unexpected rounded score: %v", score)
	}
}
