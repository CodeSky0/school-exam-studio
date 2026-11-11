package question_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	entquestion "school-exam-studio/backend/ent/question"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestRepositoryVisibilityPermissionsAndSoftDelete(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	ownDraft := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusDraft)
	ownPublished := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	otherDraft := platformtest.Question(t, client, platformtest.SchoolA, platformtest.OtherTeacherA, question.StatusDraft)
	otherPublished := platformtest.Question(t, client, platformtest.SchoolA, platformtest.OtherTeacherA, question.StatusPublished)
	foreign := platformtest.Question(t, client, platformtest.SchoolB, platformtest.TeacherB, question.StatusPublished)
	repository := question.NewRepository(client)
	teacher := platformtest.TeacherPrincipal(client)

	listed, err := repository.List(ctx, teacher, question.QuestionFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	assertQuestionSet(t, listed.Items, ownDraft.ID, ownPublished.ID, otherPublished.ID)
	if _, err := repository.Get(ctx, teacher, otherPublished.ID); err != nil {
		t.Fatalf("teacher should read another creator's published question: %v", err)
	}
	if _, err := repository.Get(ctx, teacher, otherDraft.ID); !errors.Is(err, question.ErrNotFound) {
		t.Fatalf("other draft should be invisible, got %v", err)
	}
	if _, err := repository.Get(ctx, teacher, foreign.ID); !errors.Is(err, question.ErrNotFound) {
		t.Fatalf("cross-school question should be invisible, got %v", err)
	}
	if err := repository.Delete(ctx, platformtest.TeacherBPrincipal(), ownDraft.ID); !errors.Is(err, question.ErrNotFound) {
		t.Fatalf("cross-school delete expected ErrNotFound, got %v", err)
	}
	if err := repository.Delete(ctx, teacher, otherDraft.ID); !errors.Is(err, question.ErrForbidden) {
		t.Fatalf("teacher deleting another teacher's draft expected ErrForbidden, got %v", err)
	}

	service := question.NewService(repository, question.NewSanitizer())
	if _, err := service.Update(ctx, teacher, ownPublished.ID, validQuestionInput()); !errors.Is(err, question.ErrForbidden) {
		t.Fatalf("teacher updating published question expected ErrForbidden, got %v", err)
	}
	updated, err := service.Update(ctx, teacher, ownDraft.ID, validQuestionInput())
	if err != nil || updated.Grade != "七年级" {
		t.Fatalf("teacher could not update own draft: %+v %v", updated, err)
	}
	if err := repository.Delete(ctx, teacher, ownDraft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(ctx, teacher, ownDraft.ID); !errors.Is(err, question.ErrNotFound) {
		t.Fatalf("soft-deleted question should be hidden, got %v", err)
	}
	rows, err := client.Question.Update().Where(
		entquestion.IDEQ(ownDraft.ID),
		entquestion.SchoolIDEQ(platformtest.SchoolA),
		entquestion.DeletedAtNotNil(),
	).SetStatus(question.StatusDraft).Save(ctx)
	if err != nil || rows != 1 {
		t.Fatalf("question was physically deleted or school predicate failed: rows=%d err=%v", rows, err)
	}

	adminListed, err := repository.List(ctx, platformtest.AdminPrincipal(client), question.QuestionFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if adminListed.Total != 3 {
		t.Fatalf("admin should see all remaining school questions, total=%d", adminListed.Total)
	}
}

func TestResolvePublishedAndReviewTransitions(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	draft := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusDraft)
	published := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	archived := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusArchived)
	foreign := platformtest.Question(t, client, platformtest.SchoolB, platformtest.TeacherB, question.StatusPublished)
	repository := question.NewRepository(client)
	teacher := platformtest.TeacherPrincipal(client)
	admin := platformtest.AdminPrincipal(client)

	items, err := repository.ResolvePublished(ctx, teacher, []uuid.UUID{foreign.ID, published.ID, draft.ID, archived.ID, published.ID})
	if err != nil {
		t.Fatal(err)
	}
	assertQuestionIDs(t, items, published.ID)
	if _, err := repository.UpdateStatus(ctx, teacher, draft.ID, question.StatusPublished); !errors.Is(err, question.ErrForbidden) {
		t.Fatalf("teacher review expected ErrForbidden, got %v", err)
	}
	if _, err := repository.UpdateStatus(ctx, auth.Principal{UserID: platformtest.AdminB, SchoolID: platformtest.SchoolB, Role: auth.RoleAdmin}, draft.ID, question.StatusPublished); !errors.Is(err, question.ErrNotFound) {
		t.Fatalf("cross-school review expected ErrNotFound, got %v", err)
	}
	reviewed, err := repository.UpdateStatus(ctx, admin, draft.ID, question.StatusPublished)
	if err != nil || reviewed.Status != question.StatusPublished {
		t.Fatalf("publish failed: %+v %v", reviewed, err)
	}
	service := question.NewService(repository, question.NewSanitizer())
	if _, err := service.Update(ctx, teacher, draft.ID, validQuestionInput()); !errors.Is(err, question.ErrForbidden) {
		t.Fatalf("teacher modified published question: %v", err)
	}
	if _, err := service.Update(ctx, admin, draft.ID, validQuestionInput()); !errors.Is(err, question.ErrForbidden) {
		t.Fatalf("admin modified published question: %v", err)
	}
	if err := repository.Delete(ctx, admin, draft.ID); !errors.Is(err, question.ErrForbidden) {
		t.Fatalf("admin deleted published question: %v", err)
	}
	archivedResult, err := repository.UpdateStatus(ctx, admin, draft.ID, question.StatusArchived)
	if err != nil || archivedResult.Status != question.StatusArchived {
		t.Fatalf("archive failed: %+v %v", archivedResult, err)
	}
	if _, err := repository.UpdateStatus(ctx, admin, draft.ID, question.StatusPublished); !errors.Is(err, question.ErrInvalidTransition) {
		t.Fatalf("archived question must not be republished: %v", err)
	}
}

func TestCombinedFiltersAndPagination(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	service := question.NewService(question.NewRepository(client), question.NewSanitizer())
	admin := platformtest.AdminPrincipal(client)
	created := make([]question.Question, 0, 2)
	for range 2 {
		input := validQuestionInput()
		input.Difficulty = 3
		input.Tags = []string{"代数", "方程"}
		item, err := service.Create(ctx, admin, input)
		if err != nil {
			t.Fatal(err)
		}
		item, err = service.UpdateStatus(ctx, admin, item.ID, question.StatusPublished)
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, item)
	}
	nonMatching := validQuestionInput()
	nonMatching.Difficulty = 4
	if _, err := service.Create(ctx, admin, nonMatching); err != nil {
		t.Fatal(err)
	}
	filter := question.QuestionFilter{
		Subject: "数学", Grade: "七年级", Type: question.TypeSingleChoice,
		Tag: "代数", CreatorID: platformtest.AdminA.String(), Status: question.StatusPublished,
		Difficulty: 3, Page: 1, PageSize: 1,
	}
	firstPage, err := service.List(ctx, platformtest.TeacherPrincipal(client), filter)
	if err != nil {
		t.Fatal(err)
	}
	if firstPage.Total != 2 || len(firstPage.Items) != 1 {
		t.Fatalf("unexpected combined-filter first page: %+v", firstPage)
	}
	filter.Page = 2
	secondPage, err := service.List(ctx, platformtest.TeacherPrincipal(client), filter)
	if err != nil {
		t.Fatal(err)
	}
	if secondPage.Total != 2 || len(secondPage.Items) != 1 || firstPage.Items[0].ID == secondPage.Items[0].ID {
		t.Fatalf("unstable or incorrect pagination: first=%+v second=%+v", firstPage, secondPage)
	}
	filter.Page = 1
	filter.PageSize = 1000
	capped, err := service.List(ctx, platformtest.TeacherPrincipal(client), filter)
	if err != nil || capped.PageSize != 100 || len(capped.Items) != 2 {
		t.Fatalf("page size was not capped at 100: %+v %v", capped, err)
	}
}

func assertQuestionIDs(t *testing.T, items []question.Question, expected ...uuid.UUID) {
	t.Helper()
	if len(items) != len(expected) {
		t.Fatalf("question count=%d, want %d: %+v", len(items), len(expected), items)
	}
	for index, id := range expected {
		if items[index].ID != id {
			t.Fatalf("question IDs/order=%v, want %v", questionIDs(items), expected)
		}
	}
}

func assertQuestionSet(t *testing.T, items []question.Question, expected ...uuid.UUID) {
	t.Helper()
	if len(items) != len(expected) {
		t.Fatalf("question count=%d, want %d: %+v", len(items), len(expected), items)
	}
	actual := make(map[uuid.UUID]struct{}, len(items))
	for _, item := range items {
		actual[item.ID] = struct{}{}
	}
	for _, id := range expected {
		if _, exists := actual[id]; !exists {
			t.Fatalf("question IDs=%v, missing %s", questionIDs(items), id)
		}
	}
}

func questionIDs(items []question.Question) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
