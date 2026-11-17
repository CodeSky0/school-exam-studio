package ai

import (
	"context"
	"testing"

	"school-exam-studio/backend/internal/question"
)

func TestToolCatalogIncludesExtendedTools(t *testing.T) {
	service := &Service{}
	catalog := service.ToolCatalog(testPrincipal())
	expected := map[string]string{
		"list_papers":              "paper",
		"get_paper":                "paper",
		"paper_audit":              "paper",
		"update_paper_draft":       "paper",
		"bank_stats":               "analytics",
		"find_duplicates":          "analytics",
		"review_queue":             "analytics",
		"search_similar_questions": "analytics",
	}
	found := map[string]string{}
	for _, item := range catalog {
		found[item.Name] = item.Category
	}
	for name, category := range expected {
		if found[name] != category {
			t.Fatalf("tool %s category %q, want %q", name, found[name], category)
		}
	}
}

func TestValidateToolArgumentsAcceptsNewTools(t *testing.T) {
	valid := []struct {
		name string
		args map[string]any
	}{
		{"list_papers", map[string]any{"subject": "数学", "limit": float64(10)}},
		{"bank_stats", map[string]any{"grade": "七年级"}},
		{"find_duplicates", map[string]any{}},
		{"review_queue", map[string]any{"limit": float64(20)}},
		{"search_similar_questions", map[string]any{"keyword": "分数"}},
	}
	for _, tc := range valid {
		if err := validateToolArguments(tc.name, tc.args); err != nil {
			t.Fatalf("%s rejected: %v", tc.name, err)
		}
	}
	if err := validateToolArguments("bank_stats", map[string]any{"bogus": true}); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestTextTokensAndOverlap(t *testing.T) {
	left := textTokens("计算一元一次方程 2x+1=3")
	right := textTokens("解一元一次方程 2x+1=3")
	if score := overlapScore(left, right); score <= 0 {
		t.Fatalf("expected positive overlap, got %f", score)
	}
	if score := overlapScore(left, textTokens("完全无关的英语题")); score != 0 {
		t.Fatalf("expected zero overlap, got %f", score)
	}
}

func TestReviewRequiredDefaultsToTrueForAIDrafts(t *testing.T) {
	item := question.Question{AIGenerated: true, AIMeta: map[string]any{}}
	if !reviewRequired(item) {
		t.Fatal("AI draft without explicit flag should require review")
	}
	item.AIMeta["review_required"] = false
	if reviewRequired(item) {
		t.Fatal("explicit false should skip review")
	}
	item.AIGenerated = false
	if reviewRequired(item) {
		t.Fatal("non-AI question should not be in review queue")
	}
}

func TestAssignedScoreArgument(t *testing.T) {
	if got := assignedScoreArgument(map[string]any{"assigned_score": float64(5)}); got != "5" {
		t.Fatalf("got %q", got)
	}
	if got := assignedScoreArgument(map[string]any{"assigned_score": "2.5"}); got != "2.5" {
		t.Fatalf("got %q", got)
	}
	if got := assignedScoreArgument(map[string]any{"assigned_score": float64(0)}); got != "" {
		t.Fatalf("expected empty for zero, got %q", got)
	}
}

func TestStatusExposesFallbackPolicyAndHealth(t *testing.T) {
	t.Setenv("AI_ALLOW_LOCAL_FALLBACK", "")
	status := (&Service{}).Status(context.Background(), testPrincipal())
	if status.FallbackPolicy != "error" {
		t.Fatalf("fallback policy %q", status.FallbackPolicy)
	}
	t.Setenv("AI_ALLOW_LOCAL_FALLBACK", "true")
	status = (&Service{}).Status(context.Background(), testPrincipal())
	if status.FallbackPolicy != "local" {
		t.Fatalf("fallback policy %q", status.FallbackPolicy)
	}
}

func TestListSkillsIncludesExtendedSkills(t *testing.T) {
	skills := (&Service{}).ListSkills()
	found := map[string]bool{}
	for _, skill := range skills {
		found[skill.ID] = true
	}
	for _, want := range []string{
		"bank_gap_analysis", "duplicate_cleanup", "paper_quality_check",
		"review_assistant", "exam_blueprint", "similar_question_pack",
	} {
		if !found[want] {
			t.Fatalf("missing skill %s", want)
		}
	}
}

func TestRunSkillRejectsUnknownSkill(t *testing.T) {
	service := NewService(Options{})
	if _, err := service.RunSkill(context.Background(), testPrincipal(), "unknown", SkillRunInput{}); err == nil {
		t.Fatal("expected validation error for unknown skill")
	}
}
