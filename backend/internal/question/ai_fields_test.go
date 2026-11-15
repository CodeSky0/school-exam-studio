package question

import (
	"strings"
	"testing"
)

func TestContentHashIgnoresFormatting(t *testing.T) {
	first := ContentHash(Input{
		Content: "<p>计算 1 + 1。</p>",
		Options: []Option{{Key: "A", Content: "<strong>2</strong>"}},
	})
	second := ContentHash(Input{
		Content: "<div>计算   1 + 1。</div>",
		Options: []Option{{Key: "A", Content: "2"}},
	})
	if first == "" || first != second {
		t.Fatalf("expected stable hash, got %q and %q", first, second)
	}
	third := ContentHash(Input{Content: "<p>计算 2 + 2。</p>"})
	if third == first {
		t.Fatalf("different content must have different hash")
	}
}

func TestNormalizeKnowledgePoints(t *testing.T) {
	values, err := NormalizeKnowledgePoints([]string{" 函数 ", "函数", "方程"})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != "函数" || values[1] != "方程" {
		t.Fatalf("unexpected normalized knowledge points: %v", values)
	}
	tooMany := make([]string, 9)
	for index := range tooMany {
		tooMany[index] = string(rune('A' + index))
	}
	if _, err := NormalizeKnowledgePoints(tooMany); err == nil {
		t.Fatalf("expected too many knowledge points error")
	}
}

func TestNormalizeCategory(t *testing.T) {
	value, err := NormalizeCategory("  数与运算 ")
	if err != nil || value != "数与运算" {
		t.Fatalf("unexpected category: %q %v", value, err)
	}
	if _, err := NormalizeCategory(strings.Repeat("长", 61)); err == nil {
		t.Fatalf("expected category length error")
	}
}

func TestShiftRichTextNumbers(t *testing.T) {
	content, changed := ShiftRichTextNumbers(`<p>计算 <strong>12</strong> + 7。</p>`, 1)
	if !changed {
		t.Fatalf("expected numbers to change")
	}
	if !strings.Contains(content, "<strong>13</strong>") || !strings.Contains(content, "8") {
		t.Fatalf("unexpected shifted content: %s", content)
	}
	if _, changed := ShiftRichTextNumbers("<p>没有数字。</p>", 1); changed {
		t.Fatalf("expected unchanged content")
	}
}

func TestValidateAnswerFormats(t *testing.T) {
	if _, err := ValidateAnswer(TypeMultipleChoice, []Option{{Key: "A"}, {Key: "B"}}, "AB"); err == nil {
		t.Fatalf("expected multiple choice answer format error")
	}
	answer, err := ValidateAnswer(TypeMultipleChoice, []Option{{Key: "A"}, {Key: "B"}}, "A,B")
	if err != nil || answer != "A,B" {
		t.Fatalf("unexpected multiple choice answer: %q %v", answer, err)
	}
	if _, err := ValidateAnswer(TypeTrueFalse, nil, "正确"); err == nil {
		t.Fatalf("expected true/false answer format error")
	}
	answer, err = ValidateAnswer(TypeTrueFalse, nil, "true")
	if err != nil || answer != "true" {
		t.Fatalf("unexpected true/false answer: %q %v", answer, err)
	}
}
