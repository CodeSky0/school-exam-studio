package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/question"
)

func TestLocalAnalyzeInfersMetadata(t *testing.T) {
	engine := NewLocalEngine()
	result := engine.Analyze(AnalyzeInput{
		Content: "<p>已知二次函数 y = x^2 + 1，求 x = 2 时的函数值。</p>",
		Answer:  "5",
		Grade:   "九年级",
	})
	if result.Subject != "数学" {
		t.Fatalf("expected 数学, got %q", result.Subject)
	}
	if result.Grade != "九年级" {
		t.Fatalf("expected 九年级, got %q", result.Grade)
	}
	if result.Provider != ProviderLocal {
		t.Fatalf("expected local provider, got %q", result.Provider)
	}
	if len(result.KnowledgePoints) == 0 {
		t.Fatalf("expected knowledge points")
	}
	if result.Answer != "5" {
		t.Fatalf("expected existing answer preserved, got %q", result.Answer)
	}
	if !result.ReviewRequired {
		t.Fatalf("local analysis must require review")
	}
	if result.Difficulty < 1 || result.Difficulty > 5 {
		t.Fatalf("difficulty out of range: %d", result.Difficulty)
	}
}

func TestLocalAnalyzeDoesNotInventChoiceAnswer(t *testing.T) {
	engine := NewLocalEngine()
	result := engine.Analyze(AnalyzeInput{
		Content: "<p>下列关于函数的说法正确的是（ ）。</p>",
		Options: []Option{
			{Key: "A", Content: "函数图像一定过原点"},
			{Key: "B", Content: "函数描述变量之间的关系"},
		},
	})
	if result.QuestionType != question.TypeSingleChoice {
		t.Fatalf("expected single choice, got %q", result.QuestionType)
	}
	if result.Answer != "" {
		t.Fatalf("local engine must not invent choice answer, got %q", result.Answer)
	}
	if !containsString(result.Warnings, "本地模式不会臆造客观题答案，请人工确认") {
		t.Fatalf("expected warning, got %v", result.Warnings)
	}
}

func TestLocalGenerateArithmetic(t *testing.T) {
	engine := NewLocalEngine()
	items, _ := engine.Generate(GenerateInput{
		Subject:         "数学",
		Grade:           "七年级",
		KnowledgePoints: []string{"加法"},
		Count:           2,
	})
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item.Answer == "" {
			t.Fatalf("arithmetic item must include answer")
		}
		if !strings.Contains(item.Content, "+") {
			t.Fatalf("expected addition content, got %q", item.Content)
		}
		if !item.ReviewRequired {
			t.Fatalf("generated item must require review")
		}
	}
}

func TestLocalVariantsShiftNumbersAndKeepMarkup(t *testing.T) {
	engine := NewLocalEngine()
	items, _ := engine.Variants(QuestionSnapshot{
		Content:      "<p>计算 <strong>12</strong> + 7 的值。</p>",
		QuestionType: question.TypeFillBlank,
		Difficulty:   2,
		Category:     "数与运算",
		Tags:         []string{"加法"},
	}, VariantInput{Count: 1})
	if len(items) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(items))
	}
	if !strings.Contains(items[0].Content, "<strong>13</strong>") {
		t.Fatalf("expected shifted number in markup, got %q", items[0].Content)
	}
	if items[0].Answer == "" {
		t.Fatalf("variant draft must include reviewable answer placeholder")
	}
}

func TestLocalAuditDetectsDuplicatesAndMissingFields(t *testing.T) {
	engine := NewLocalEngine()
	result := engine.Audit([]QuestionSnapshot{
		{ID: "q1", Content: "<p>计算 1 + 1。</p>", Difficulty: 1, Status: question.StatusDraft},
		{ID: "q2", Content: "<p>计算 1 + 1。</p>", Difficulty: 1, Status: question.StatusDraft, Answer: "2"},
	})
	if result.Total != 2 {
		t.Fatalf("expected total 2, got %d", result.Total)
	}
	if result.DuplicateCount != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", result.DuplicateCount)
	}
	if result.MissingAnswer != 1 || result.MissingExplanation != 2 || result.MissingKnowledgePoints != 2 {
		t.Fatalf("unexpected missing counts: %+v", result)
	}
}

func TestPlanGoalIntents(t *testing.T) {
	cases := []struct {
		goal     string
		question string
		intent   string
		count    int
	}{
		{"帮我给数学草稿补标签和解析，共5道", "", IntentOrganizeBank, 5},
		{"为七年级「二次函数」生成3道变式题", "", IntentGenerateVariants, 3},
		{"出一份八年级数学试卷，10道题", "", IntentComposePaper, 10},
		{"帮我体检题库", "", IntentAuditBank, 0},
		{"对当前题库做一次质量体检，找出重复题和缺少解析的题目", "", IntentAuditBank, 0},
		{"整理题库", "", IntentTagQuestions, 0},
		{"分析这道题", uuid.NewString(), IntentAnalyzeQuestion, 0},
		{"你好", "", IntentUnknown, 0},
	}
	for _, testCase := range cases {
		plan, err := PlanGoal(AgentRequest{Goal: testCase.goal, QuestionID: testCase.question})
		if err != nil {
			t.Fatalf("plan %q: %v", testCase.goal, err)
		}
		if plan.Intent != testCase.intent {
			t.Fatalf("goal %q: expected %s, got %s", testCase.goal, testCase.intent, plan.Intent)
		}
		if plan.Params.Count != testCase.count {
			t.Fatalf("goal %q: expected count %d, got %d", testCase.goal, testCase.count, plan.Params.Count)
		}
	}
}

func TestLocalVariantsKeepValidChoiceAnswer(t *testing.T) {
	engine := NewLocalEngine()
	items, _ := engine.Variants(QuestionSnapshot{
		Subject:      "数学",
		Grade:        "八年级",
		QuestionType: question.TypeSingleChoice,
		Difficulty:   2,
		Content:      "<p>已知 3 + 5 = ?</p>",
		Options: []Option{
			{Key: "A", Content: "6"},
			{Key: "B", Content: "8"},
		},
		Answer: "B",
	}, VariantInput{Count: 1})
	if len(items) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(items))
	}
	if _, err := question.ValidateAnswer(question.TypeSingleChoice, toQuestionOptions(items[0].Options), items[0].Answer); err != nil {
		t.Fatalf("variant answer must remain valid for objective questions: %v", err)
	}
}

func TestLocalGenerateFillBlankSerializesOptionsAsArray(t *testing.T) {
	items, _ := NewLocalEngine().Generate(GenerateInput{
		Subject:         "数学",
		Grade:           "八年级",
		QuestionType:    question.TypeFillBlank,
		Count:           1,
		KnowledgePoints: []string{"函数"},
	})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	encoded, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"options":[]`) {
		t.Fatalf("empty options must serialize as [], got %s", encoded)
	}
}

func TestPlanGoalExtractsKnowledgeAndGrade(t *testing.T) {
	plan, err := PlanGoal(AgentRequest{Goal: "为七年级「二次函数」生成3道变式题"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Params.Grade != "七年级" {
		t.Fatalf("expected 七年级, got %q", plan.Params.Grade)
	}
	if plan.Params.KnowledgePoint != "二次函数" {
		t.Fatalf("expected 二次函数, got %q", plan.Params.KnowledgePoint)
	}
}

func TestServiceAnalyzeFallsBackWhenProviderFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	service := NewService(Options{
		LLMBaseURL: server.URL,
		LLMAPIKey:  "test-key",
		LLMModel:   "test-model",
		LLMTimeout: 5 * time.Second,
	})
	result, err := service.Analyze(context.Background(), testPrincipal(), AnalyzeInput{
		Content: "<p>计算 1 + 1。</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != ProviderFallback {
		t.Fatalf("expected fallback provider, got %q", result.Provider)
	}
	if !containsString(result.Warnings, "大模型调用失败，已自动降级为本地引擎") {
		t.Fatalf("expected fallback warning, got %v", result.Warnings)
	}
}

func TestServiceAnalyzeUsesProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"` + "```json\\n" + `{\"subject\":\"数学\",\"grade\":\"七年级\",\"question_type\":\"fill_blank\",\"difficulty\":2,\"category\":\"数与运算\",\"tags\":[\"加法\"],\"knowledge_points\":[\"有理数加法\"],\"answer\":\"2\",\"explanation\":\"<p>1+1=2。</p>\",\"confidence\":0.9,\"rationale\":\"测试\"}` + "\\n```" + `"}}]}`))
	}))
	defer server.Close()
	service := NewService(Options{
		LLMBaseURL: server.URL,
		LLMAPIKey:  "test-key",
		LLMModel:   "test-model",
		LLMTimeout: 5 * time.Second,
	})
	result, err := service.Analyze(context.Background(), testPrincipal(), AnalyzeInput{
		Content: "<p>计算 1 + 1。</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != ProviderLLM {
		t.Fatalf("expected llm provider, got %q", result.Provider)
	}
	if result.Model != "test-model" {
		t.Fatalf("expected model, got %q", result.Model)
	}
	if result.Answer != "2" || result.Category != "数与运算" {
		t.Fatalf("unexpected analysis: %+v", result)
	}
}

func testPrincipal() auth.Principal {
	return auth.Principal{
		UserID:   uuid.New(),
		SchoolID: uuid.New(),
		Role:     auth.RoleTeacher,
		Subject:  "数学",
	}
}
