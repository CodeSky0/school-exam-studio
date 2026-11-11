package admin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"school-exam-studio/backend/ent"
	entuser "school-exam-studio/backend/ent/user"
	"school-exam-studio/backend/internal/admin"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestAdministrationTenantIsolationSessionsAndStats(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	authService := auth.NewService(auth.NewRepository(client), auth.NewPasswordHasher(bcrypt.MinCost), "01234567890123456789012345678901", time.Hour)
	service := admin.NewService(admin.NewRepository(client), auth.NewPasswordHasher(bcrypt.MinCost))
	adminPrincipal := platformtest.AdminPrincipal(client)
	teacherPrincipal := platformtest.TeacherPrincipal(client)

	listed, err := service.ListTeachers(ctx, adminPrincipal, admin.TeacherFilter{Subject: "数学", Status: auth.StatusActive, Page: 1, PageSize: 20})
	if err != nil || listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].ID != platformtest.TeacherA {
		t.Fatalf("unexpected filtered teachers: %+v %v", listed, err)
	}
	if _, err := service.ListTeachers(ctx, teacherPrincipal, admin.TeacherFilter{}); !errors.Is(err, admin.ErrForbidden) {
		t.Fatalf("teacher administration should be forbidden: %v", err)
	}
	if _, err := service.UpdateTeacherStatus(ctx, auth.Principal{UserID: platformtest.AdminB, SchoolID: platformtest.SchoolB, Role: auth.RoleAdmin}, platformtest.TeacherA, auth.StatusDisabled); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("cross-school status update should be not found: %v", err)
	}
	foreign, err := client.User.Get(ctx, platformtest.TeacherA)
	if err != nil || foreign.SessionVersion != 1 {
		t.Fatalf("cross-school update changed teacher: %+v %v", foreign, err)
	}

	disabled, err := service.UpdateTeacherStatus(ctx, adminPrincipal, platformtest.TeacherA, auth.StatusDisabled)
	if err != nil || disabled.Status != auth.StatusDisabled {
		t.Fatalf("disable teacher failed: %+v %v", disabled, err)
	}
	assertSessionVersion(t, client, platformtest.TeacherA, 2)
	if _, err := authService.Authenticate(ctx, teacherPrincipal); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("disabled teacher session should be invalid: %v", err)
	}
	unchanged, err := service.UpdateTeacherStatus(ctx, adminPrincipal, platformtest.TeacherA, auth.StatusDisabled)
	if err != nil || unchanged.Status != auth.StatusDisabled {
		t.Fatal(err)
	}
	assertSessionVersion(t, client, platformtest.TeacherA, 2)
	if _, err := service.UpdateTeacherStatus(ctx, adminPrincipal, platformtest.TeacherA, auth.StatusActive); err != nil {
		t.Fatal(err)
	}
	assertSessionVersion(t, client, platformtest.TeacherA, 3)
	beforeReset := teacherPrincipal
	beforeReset.SessionVersion = 3
	temporaryPassword, err := service.ResetTeacherPassword(ctx, adminPrincipal, platformtest.TeacherA)
	if err != nil || len(temporaryPassword) < 16 {
		t.Fatalf("temporary password length=%d err=%v", len(temporaryPassword), err)
	}
	assertSessionVersion(t, client, platformtest.TeacherA, 4)
	if _, err := authService.Authenticate(ctx, beforeReset); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("password reset must invalidate an active old session: %v", err)
	}
	resetTeacher, err := client.User.Get(ctx, platformtest.TeacherA)
	if err != nil || !auth.NewPasswordHasher(bcrypt.MinCost).Verify(resetTeacher.PasswordHash, temporaryPassword) {
		t.Fatalf("temporary password was not bcrypt hashed: %v", err)
	}
	if _, err := service.ResetTeacherPassword(ctx, auth.Principal{UserID: platformtest.AdminB, SchoolID: platformtest.SchoolB, Role: auth.RoleAdmin}, platformtest.TeacherA); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("cross-school reset should be not found: %v", err)
	}
	if _, err := service.UpdateTeacherStatus(ctx, adminPrincipal, platformtest.OtherTeacherA, auth.StatusDisabled); err != nil {
		t.Fatal(err)
	}

	school, err := service.UpdateSchool(ctx, adminPrincipal, "  新的甲学校  ")
	if err != nil || school.Name != "新的甲学校" || school.SchoolCode != "00000001" {
		t.Fatalf("school update failed: %+v %v", school, err)
	}
	if _, err := service.UpdateSchool(ctx, adminPrincipal, ""); !errors.Is(err, admin.ErrValidation) {
		t.Fatalf("empty school name should fail: %v", err)
	}

	createAdminStatFixtures(t, client)
	stats, err := service.Stats(ctx, adminPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Teachers.Active != 1 || stats.Teachers.Disabled != 1 ||
		stats.Questions.Draft != 2 || stats.Questions.Published != 1 || stats.Questions.Archived != 1 || stats.Papers != 2 {
		t.Fatalf("unexpected admin stats: %+v", stats)
	}
	teacherStats, err := service.TeacherStats(ctx, teacherPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	if teacherStats.Questions.Draft != 1 || teacherStats.Questions.Published != 1 || teacherStats.Questions.Archived != 1 || teacherStats.Papers != 1 {
		t.Fatalf("unexpected teacher stats: %+v", teacherStats)
	}
	if _, err := service.TeacherStats(ctx, adminPrincipal); !errors.Is(err, admin.ErrForbidden) {
		t.Fatalf("admin cannot use teacher-only stats endpoint: %v", err)
	}
}

func assertSessionVersion(t *testing.T, client *ent.Client, userID uuid.UUID, expected int) {
	t.Helper()
	entity, err := client.User.Query().Where(entuser.IDEQ(userID)).Only(context.Background())
	if err != nil {
		t.Fatalf("query session version: %v", err)
	}
	if entity.SessionVersion != expected {
		t.Fatalf("session version=%d, want %d", entity.SessionVersion, expected)
	}
}

func createAdminStatFixtures(t *testing.T, client *ent.Client) {
	t.Helper()
	ctx := context.Background()
	for _, status := range []string{question.StatusDraft, question.StatusPublished, question.StatusArchived} {
		platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, status)
	}
	platformtest.Question(t, client, platformtest.SchoolA, platformtest.OtherTeacherA, question.StatusDraft)
	platformtest.Question(t, client, platformtest.SchoolB, platformtest.TeacherB, question.StatusPublished)
	deletedQuestion := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	if _, err := client.Question.UpdateOneID(deletedQuestion.ID).SetDeletedAt(time.Now()).Save(ctx); err != nil {
		t.Fatal(err)
	}
	createPaper := func(schoolID, creatorID uuid.UUID) *ent.Paper {
		entity, err := client.Paper.Create().SetSchoolID(schoolID).SetCreatorID(creatorID).
			SetTitle("统计试卷").SetSubject("数学").SetGrade("七年级").SetTotalScore("100.00").SetDurationMinutes(90).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return entity
	}
	createPaper(platformtest.SchoolA, platformtest.TeacherA)
	createPaper(platformtest.SchoolA, platformtest.OtherTeacherA)
	createPaper(platformtest.SchoolB, platformtest.TeacherB)
	deletedPaper := createPaper(platformtest.SchoolA, platformtest.TeacherA)
	if _, err := client.Paper.UpdateOneID(deletedPaper.ID).SetDeletedAt(time.Now()).Save(ctx); err != nil {
		t.Fatal(err)
	}
}
