package paper_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	entpaper "school-exam-studio/backend/ent/paper"
	entpaperquestion "school-exam-studio/backend/ent/paperquestion"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/paper"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestServiceRejectsInvalidQuestionsAndRollsBackTransaction(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	service := paper.NewService(paper.NewRepository(client))
	teacher := platformtest.TeacherPrincipal(client)
	published := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	secondPublished := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	draft := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusDraft)
	archived := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusArchived)
	foreign := platformtest.Question(t, client, platformtest.SchoolB, platformtest.TeacherB, question.StatusPublished)
	mismatched := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	if _, err := mismatched.Update().SetSubject("语文").Save(ctx); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input paper.Input
		want  error
	}{
		{name: "draft", input: inputWithQuestions(published.ID, draft.ID), want: paper.ErrQuestionUnavailable},
		{name: "foreign school", input: inputWithQuestions(foreign.ID), want: paper.ErrQuestionUnavailable},
		{name: "archived for new paper", input: inputWithQuestions(archived.ID), want: paper.ErrQuestionUnavailable},
		{name: "subject mismatch", input: inputWithQuestions(mismatched.ID), want: paper.ErrQuestionMismatch},
		{name: "duplicate", input: inputWithQuestions(published.ID, published.ID), want: paper.ErrDuplicateQuestion},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Create(ctx, teacher, test.input); !errors.Is(err, test.want) {
				t.Fatalf("Create error = %v, want %v", err, test.want)
			}
			assertActivePaperRows(t, client, 0, 0)
		})
	}

	invalidInputs := []paper.Input{
		{Title: "练习", Subject: "数学", Grade: "七年级", DurationMinutes: 60},
		inputWithScore(published.ID, "0.00"),
		inputWithScore(published.ID, "0.001"),
	}
	invalidDuration := inputWithQuestions(published.ID)
	invalidDuration.DurationMinutes = 601
	invalidInputs = append(invalidInputs, invalidDuration)
	for _, input := range invalidInputs {
		if _, err := service.Create(ctx, teacher, input); !errors.Is(err, paper.ErrValidation) {
			t.Fatalf("invalid input error = %v, want ErrValidation", err)
		}
		assertActivePaperRows(t, client, 0, 0)
	}

	overflow := inputWithQuestions(published.ID, secondPublished.ID)
	for index := range overflow.Questions {
		overflow.Questions[index].AssignedScore = paper.MaximumScore
	}
	if _, err := service.Create(ctx, teacher, overflow); !errors.Is(err, paper.ErrValidation) {
		t.Fatalf("total overflow error = %v, want ErrValidation", err)
	}
	// The total is checked after paper and detail writes inside the transaction.
	// Both kinds of rows must therefore be absent after the rollback.
	assertActivePaperRows(t, client, 0, 0)
}

func TestServiceCreateUsesExactScores(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	service := paper.NewService(paper.NewRepository(client))
	first := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	second := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	input := inputWithQuestions(first.ID, second.ID)
	input.Questions[0].AssignedScore = "0.10"
	input.Questions[1].AssignedScore = "0.20"

	created, err := service.Create(context.Background(), platformtest.TeacherPrincipal(client), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.TotalScore != "0.30" || created.SchoolName != "甲学校" || created.Title != "七年级数学练习" {
		t.Fatalf("unexpected paper: %+v", created)
	}
	if len(created.Questions) != 2 {
		t.Fatalf("question count = %d, want 2", len(created.Questions))
	}
	for index, id := range []uuid.UUID{first.ID, second.ID} {
		detail := created.Questions[index]
		if detail.QuestionID != id || detail.OrderIndex != index || detail.Question.ID != id {
			t.Fatalf("detail %d = %+v", index, detail)
		}
	}
	if created.Questions[0].AssignedScore != "0.10" || created.Questions[1].AssignedScore != "0.20" {
		t.Fatalf("assigned scores were not preserved: %+v", created.Questions)
	}
	assertActivePaperRows(t, client, 1, 2)
}

func TestServiceUpdateSoftDeletesDetailsAndKeepsArchivedHistory(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	service := paper.NewService(paper.NewRepository(client))
	teacher := platformtest.TeacherPrincipal(client)
	first := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	second := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	third := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	created, err := service.Create(ctx, teacher, inputWithQuestions(first.ID, second.ID))
	if err != nil {
		t.Fatal(err)
	}
	otherTeacher := auth.Principal{UserID: platformtest.OtherTeacherA, SchoolID: platformtest.SchoolA, Role: auth.RoleTeacher}
	if _, err := service.Get(ctx, otherTeacher, created.ID); !errors.Is(err, paper.ErrForbidden) {
		t.Fatalf("other teacher Get error = %v, want ErrForbidden", err)
	}
	if _, err := service.Update(ctx, otherTeacher, created.ID, inputWithQuestions(first.ID)); !errors.Is(err, paper.ErrForbidden) {
		t.Fatalf("other teacher Update error = %v, want ErrForbidden", err)
	}
	if err := service.Delete(ctx, otherTeacher, created.ID); !errors.Is(err, paper.ErrForbidden) {
		t.Fatalf("other teacher Delete error = %v, want ErrForbidden", err)
	}
	if _, err := service.Get(ctx, platformtest.TeacherBPrincipal(), created.ID); !errors.Is(err, paper.ErrNotFound) {
		t.Fatalf("cross-school Get error = %v, want ErrNotFound", err)
	}
	if _, err := service.Update(ctx, platformtest.TeacherBPrincipal(), created.ID, inputWithQuestions(first.ID)); !errors.Is(err, paper.ErrNotFound) {
		t.Fatalf("cross-school Update error = %v, want ErrNotFound", err)
	}
	if err := service.Delete(ctx, platformtest.TeacherBPrincipal(), created.ID); !errors.Is(err, paper.ErrNotFound) {
		t.Fatalf("cross-school Delete error = %v, want ErrNotFound", err)
	}
	if _, err := service.Get(ctx, platformtest.AdminPrincipal(client), created.ID); err != nil {
		t.Fatalf("school admin should read teacher paper: %v", err)
	}

	updatedInput := inputWithQuestions(third.ID, first.ID)
	updatedInput.Title = "更新后的试卷"
	updatedInput.DurationMinutes = 90
	updatedInput.Questions[0].AssignedScore = "0.20"
	updatedInput.Questions[1].AssignedScore = "0.10"
	updated, err := service.Update(ctx, teacher, created.ID, updatedInput)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TotalScore != "0.30" || updated.Title != updatedInput.Title || updated.DurationMinutes != 90 {
		t.Fatalf("unexpected updated paper: %+v", updated)
	}
	for index, expected := range []struct {
		id    uuid.UUID
		score string
	}{{third.ID, "0.20"}, {first.ID, "0.10"}} {
		detail := updated.Questions[index]
		if detail.QuestionID != expected.id || detail.OrderIndex != index || detail.AssignedScore != expected.score {
			t.Fatalf("updated detail %d = %+v", index, detail)
		}
	}
	assertActivePaperRows(t, client, 1, 2)
	deletedDetails, err := client.PaperQuestion.Update().Where(
		entpaperquestion.SchoolIDEQ(platformtest.SchoolA),
		entpaperquestion.PaperIDEQ(created.ID),
		entpaperquestion.DeletedAtNotNil(),
	).SetDeletedAt(time.Now().UTC()).Save(ctx)
	if err != nil || deletedDetails != 2 {
		t.Fatalf("soft-deleted old details = %d, want 2 (err=%v)", deletedDetails, err)
	}

	overflowInput := inputWithQuestions(third.ID, first.ID)
	for index := range overflowInput.Questions {
		overflowInput.Questions[index].AssignedScore = paper.MaximumScore
	}
	if _, err := service.Update(ctx, teacher, created.ID, overflowInput); !errors.Is(err, paper.ErrValidation) {
		t.Fatalf("update total overflow error = %v, want ErrValidation", err)
	}
	afterRollback, err := service.Get(ctx, teacher, created.ID)
	if err != nil || afterRollback.TotalScore != "0.30" || afterRollback.Questions[0].QuestionID != third.ID || afterRollback.Questions[1].QuestionID != first.ID {
		t.Fatalf("failed update changed committed paper: %+v, err=%v", afterRollback, err)
	}
	assertActivePaperRows(t, client, 1, 2)

	if _, err := first.Update().SetStatus(question.StatusArchived).Save(ctx); err != nil {
		t.Fatal(err)
	}
	historical, err := service.Get(ctx, teacher, created.ID)
	if err != nil {
		t.Fatalf("archived question must remain readable in historical paper: %v", err)
	}
	if historical.Questions[1].Question.Status != question.StatusArchived || historical.Questions[1].Question.Content == "" {
		t.Fatalf("archived question was not fully loaded: %+v", historical.Questions[1])
	}
	if _, err := service.Update(ctx, teacher, created.ID, updatedInput); !errors.Is(err, paper.ErrQuestionUnavailable) {
		t.Fatalf("archived question in a new update error = %v, want ErrQuestionUnavailable", err)
	}

	if err := service.Delete(ctx, teacher, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(ctx, teacher, created.ID); !errors.Is(err, paper.ErrNotFound) {
		t.Fatalf("deleted paper Get error = %v, want ErrNotFound", err)
	}
	deletedPapers, err := client.Paper.Update().Where(
		entpaper.SchoolIDEQ(platformtest.SchoolA),
		entpaper.IDEQ(created.ID),
		entpaper.DeletedAtNotNil(),
	).SetDeletedAt(time.Now().UTC()).Save(ctx)
	if err != nil || deletedPapers != 1 {
		t.Fatalf("paper was not soft deleted: rows=%d err=%v", deletedPapers, err)
	}
}

func TestServiceListFiltersStablePaginationAndCapsPageSize(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	service := paper.NewService(paper.NewRepository(client))
	teacher := platformtest.TeacherPrincipal(client)
	published := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	first, err := service.Create(ctx, teacher, inputWithQuestions(published.ID))
	if err != nil {
		t.Fatal(err)
	}
	secondInput := inputWithQuestions(published.ID)
	secondInput.Title = "第二份试卷"
	second, err := service.Create(ctx, teacher, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	otherPrincipal := auth.Principal{UserID: platformtest.OtherTeacherA, SchoolID: platformtest.SchoolA, Role: auth.RoleTeacher}
	if _, err := service.Create(ctx, otherPrincipal, inputWithQuestions(published.ID)); err != nil {
		t.Fatal(err)
	}

	fixedTime := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if _, err := client.Paper.Update().Where(
		entpaper.SchoolIDEQ(platformtest.SchoolA),
		entpaper.CreatorIDEQ(platformtest.TeacherA),
	).SetUpdatedAt(fixedTime).Save(ctx); err != nil {
		t.Fatal(err)
	}
	expectedIDs := []uuid.UUID{first.ID, second.ID}
	sort.Slice(expectedIDs, func(i, j int) bool { return expectedIDs[i].String() > expectedIDs[j].String() })
	for pageIndex, expectedID := range expectedIDs {
		listed, err := service.List(ctx, teacher, paper.Filter{Subject: "数学", Grade: "七年级", Page: pageIndex + 1, PageSize: 1})
		if err != nil {
			t.Fatal(err)
		}
		if listed.Total != 2 || len(listed.Items) != 1 || listed.Items[0].ID != expectedID || listed.Items[0].Questions != nil {
			t.Fatalf("page %d = %+v, want ID %s", pageIndex+1, listed, expectedID)
		}
	}
	capped, err := service.List(ctx, teacher, paper.Filter{Page: 1, PageSize: 1000})
	if err != nil || capped.PageSize != 100 || capped.Total != 2 {
		t.Fatalf("page size cap result = %+v, err=%v", capped, err)
	}
	adminListed, err := service.List(ctx, platformtest.AdminPrincipal(client), paper.Filter{Page: 1, PageSize: 20})
	if err != nil || adminListed.Total != 3 {
		t.Fatalf("admin school-wide list = %+v, err=%v", adminListed, err)
	}
}

func inputWithQuestions(ids ...uuid.UUID) paper.Input {
	input := paper.Input{
		Title:           "  七年级数学练习  ",
		Subject:         "数学",
		Grade:           "七年级",
		DurationMinutes: 60,
		Questions:       make([]paper.QuestionInput, 0, len(ids)),
	}
	for _, id := range ids {
		input.Questions = append(input.Questions, paper.QuestionInput{QuestionID: id, AssignedScore: "1.00"})
	}
	return input
}

func inputWithScore(id uuid.UUID, score string) paper.Input {
	input := inputWithQuestions(id)
	input.Questions[0].AssignedScore = score
	return input
}

func assertActivePaperRows(t *testing.T, client *ent.Client, papers, details int) {
	t.Helper()
	ctx := context.Background()
	paperCount, err := client.Paper.Query().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	detailCount, err := client.PaperQuestion.Query().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if paperCount != papers || detailCount != details {
		t.Fatalf("active paper rows = (%d, %d), want (%d, %d)", paperCount, detailCount, papers, details)
	}
}
