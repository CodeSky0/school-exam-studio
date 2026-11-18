package ai

import (
	"testing"

	"school-exam-studio/backend/internal/question"
)

func TestUniqueStringsIgnoresCaseAndWhitespace(t *testing.T) {
	got := uniqueStrings([]string{"ADD", " add ", "加法", "加 法", "加法"})
	want := []string{"ADD", "加法"}
	if len(got) != len(want) {
		t.Fatalf("uniqueStrings = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("uniqueStrings = %#v, want %#v", got, want)
		}
	}
}

func TestLocalGenerateTagsDoNotRepeatCategoryOrKnowledgePoints(t *testing.T) {
	engine := NewLocalEngine()
	items, _ := engine.Generate(GenerateInput{
		Subject:         "数学",
		Grade:           "七年级",
		Category:        "数与运算",
		KnowledgePoints: []string{"有理数加法"},
		Tags:            []string{"数与运算", "有理数加法", "口算"},
		Count:           2,
	})
	if len(items) == 0 {
		t.Fatal("expected generated items")
	}
	for _, item := range items {
		assertTagsExcludeMetadata(t, item.Tags, item.Category, item.KnowledgePoints)
	}
}

func TestNormalizeGeneratedTagsDoNotRepeatCategoryOrKnowledgePoints(t *testing.T) {
	service := &Service{sanitizer: question.NewSanitizer()}
	items := service.normalizeGenerated([]llmItem{{
		Content:         "<p>1+1=?</p>",
		Options:         []Option{{Key: "A", Content: "1"}, {Key: "B", Content: "2"}},
		Answer:          "B",
		Explanation:     "<p>基础加法</p>",
		QuestionType:    question.TypeSingleChoice,
		Difficulty:      2,
		Category:        "数与运算",
		KnowledgePoints: []string{"有理数加法"},
		Tags:            []string{"数与运算", " 有理数加法 ", "加法", "加法", "ADD", "add"},
	}}, GenerateInput{Subject: "数学", Grade: "七年级", Count: 1})
	if len(items) != 1 {
		t.Fatalf("normalizeGenerated = %#v", items)
	}
	assertTagsExcludeMetadata(t, items[0].Tags, items[0].Category, items[0].KnowledgePoints)
	seen := map[string]bool{}
	for _, tag := range items[0].Tags {
		key := metadataKey(tag)
		if seen[key] {
			t.Fatalf("duplicate tag %q in %#v", tag, items[0].Tags)
		}
		seen[key] = true
	}
}

func TestAnalyzeTagsDoNotRepeatCategoryOrKnowledgePoints(t *testing.T) {
	engine := NewLocalEngine()
	analysis := engine.Analyze(AnalyzeInput{
		Content:         "<p>计算 1+1。</p>",
		Subject:         "数学",
		Grade:           "七年级",
		Category:        "数与运算",
		KnowledgePoints: []string{"有理数加法"},
		Tags:            []string{"数与运算", "有理数加法", "口算"},
	})
	assertTagsExcludeMetadata(t, analysis.Tags, analysis.Category, analysis.KnowledgePoints)
}

func assertTagsExcludeMetadata(t *testing.T, tags []string, category string, knowledgePoints []string) {
	t.Helper()
	excluded := map[string]bool{}
	if key := metadataKey(category); key != "" {
		excluded[key] = true
	}
	for _, point := range knowledgePoints {
		if key := metadataKey(point); key != "" {
			excluded[key] = true
		}
	}
	for _, tag := range tags {
		if excluded[metadataKey(tag)] {
			t.Fatalf("tag %q duplicates category or knowledge point in %#v", tag, tags)
		}
	}
}
