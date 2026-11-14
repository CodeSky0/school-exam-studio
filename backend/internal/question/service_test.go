package question_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestCreateSanitizesRichTextAndEnforcesCreatorAndSubject(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	service := question.NewService(question.NewRepository(client), question.NewSanitizer())
	input := validQuestionInput()
	input.Content = `<p>已知<script>alert(1)</script><img src=x onerror=alert(1)><span data-type="inlineMath" data-latex="x^2"></span></p>`
	input.Options[0].Content = `<strong onclick="alert(1)">选项</strong><iframe src="https://evil.example"></iframe>`
	input.Explanation = `<p>解析</p><script>alert(2)</script>`
	created, err := service.Create(context.Background(), platformtest.TeacherPrincipal(client), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatorID != platformtest.TeacherA || created.CreatorName != "甲校数学教师" || created.Status != question.StatusDraft {
		t.Fatalf("server-owned fields were not enforced: %+v", created)
	}
	for _, value := range []string{created.Content, created.Options[0].Content, created.Explanation} {
		for _, forbidden := range []string{"<script", "<img", "<iframe", "onclick", "onerror", "evil.example"} {
			if strings.Contains(value, forbidden) {
				t.Fatalf("unsafe HTML persisted in %q", value)
			}
		}
	}
	if !strings.Contains(created.Content, `data-type="inlineMath"`) || !strings.Contains(created.Content, `data-latex="x^2"`) {
		t.Fatalf("math nodes were not preserved: %s", created.Content)
	}

	wrongSubject := validQuestionInput()
	wrongSubject.Subject = "物理"
	if _, err := service.Create(context.Background(), platformtest.TeacherPrincipal(client), wrongSubject); !errors.Is(err, question.ErrSubjectMismatch) {
		t.Fatalf("expected subject mismatch, got %v", err)
	}
	adminInput := validQuestionInput()
	adminInput.Subject = "物理"
	adminCreated, err := service.Create(context.Background(), platformtest.AdminPrincipal(client), adminInput)
	if err != nil {
		t.Fatalf("admin should be able to create any subject: %v", err)
	}
	if adminCreated.CreatorID != platformtest.AdminA || adminCreated.Status != question.StatusDraft {
		t.Fatalf("unexpected admin-created question: %+v", adminCreated)
	}
}

func TestFillBlankAndEssayAnswersAreSanitized(t *testing.T) {
	sanitizer := question.NewSanitizer()
	for _, questionType := range []string{question.TypeFillBlank, question.TypeEssay} {
		input := validQuestionInput()
		input.QuestionType = questionType
		input.Options = nil
		input.Answer = `<p>正确<script>alert(1)</script>答案</p>`
		normalized, err := sanitizer.Normalize(input)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(normalized.Answer, "script") || !strings.Contains(normalized.Answer, "正确答案") {
			t.Fatalf("unexpected sanitized answer: %s", normalized.Answer)
		}
	}
}

func TestListRejectsOverflowingPagination(t *testing.T) {
	service := question.NewService(nil, question.NewSanitizer())
	_, err := service.List(context.Background(), auth.Principal{}, question.QuestionFilter{
		Page:     math.MaxInt,
		PageSize: 100,
	})
	if !errors.Is(err, question.ErrValidation) {
		t.Fatalf("expected overflow pagination rejection, got %v", err)
	}
}
