package platformtest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	entquestion "school-exam-studio/backend/ent/question"
	"school-exam-studio/backend/ent/schema"
	entschool "school-exam-studio/backend/ent/school"
	entuser "school-exam-studio/backend/ent/user"
	"school-exam-studio/backend/internal/auth"
)

var (
	SchoolA       = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	SchoolB       = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	TeacherA      = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	TeacherB      = uuid.MustParse("00000000-0000-4000-8000-000000000102")
	AdminA        = uuid.MustParse("00000000-0000-4000-8000-000000000103")
	OtherTeacherA = uuid.MustParse("00000000-0000-4000-8000-000000000104")
	AdminB        = uuid.MustParse("00000000-0000-4000-8000-000000000105")
)

func EnsureFixtures(t *testing.T, client *ent.Client) {
	t.Helper()
	ctx := context.Background()
	schools := []struct {
		id   uuid.UUID
		name string
		code string
	}{
		{SchoolA, "甲学校", "00000001"},
		{SchoolB, "乙学校", "00000002"},
	}
	for _, fixture := range schools {
		exists, err := client.School.Query().Where(entschool.IDEQ(fixture.id)).Exist(ctx)
		if err != nil {
			t.Fatalf("check school fixture: %v", err)
		}
		if exists {
			continue
		}
		if _, err := client.School.Create().SetID(fixture.id).SetName(fixture.name).SetSchoolCode(fixture.code).Save(ctx); err != nil {
			t.Fatalf("create school fixture: %v", err)
		}
	}
	users := []struct {
		id       uuid.UUID
		schoolID uuid.UUID
		role     string
		name     string
		subject  string
		email    string
	}{
		{TeacherA, SchoolA, auth.RoleTeacher, "甲校数学教师", "数学", "teacher-a@example.com"},
		{TeacherB, SchoolB, auth.RoleTeacher, "乙校数学教师", "数学", "teacher-b@example.com"},
		{AdminA, SchoolA, auth.RoleAdmin, "甲校管理员", "", "admin-a@example.com"},
		{OtherTeacherA, SchoolA, auth.RoleTeacher, "甲校语文教师", "语文", "other-a@example.com"},
		{AdminB, SchoolB, auth.RoleAdmin, "乙校管理员", "", "admin-b@example.com"},
	}
	for _, fixture := range users {
		exists, err := client.User.Query().Where(entuser.IDEQ(fixture.id)).Exist(ctx)
		if err != nil {
			t.Fatalf("check user fixture: %v", err)
		}
		if exists {
			continue
		}
		_, err = client.User.Create().
			SetID(fixture.id).
			SetSchoolID(fixture.schoolID).
			SetRole(fixture.role).
			SetName(fixture.name).
			SetSubject(fixture.subject).
			SetEmail(fixture.email).
			SetPasswordHash("fixture-password-hash").
			SetStatus(auth.StatusActive).
			SetSessionVersion(1).
			Save(ctx)
		if err != nil && !ent.IsConstraintError(err) {
			t.Fatalf("create user fixture: %v", err)
		}
	}
}

func Question(t *testing.T, client *ent.Client, schoolID, creatorID uuid.UUID, status string) *ent.Question {
	t.Helper()
	EnsureFixtures(t, client)
	entity, err := client.Question.Create().
		SetSchoolID(schoolID).
		SetCreatorID(creatorID).
		SetSubject("数学").
		SetGrade("七年级").
		SetQuestionType("single_choice").
		SetDifficulty(1).
		SetTags([]string{"代数"}).
		SetContent("<p>1+1=?</p>").
		SetOptions([]schema.Option{{Key: "A", Content: "1"}, {Key: "B", Content: "2"}}).
		SetAnswer("B").
		SetExplanation("<p>基础加法</p>").
		SetStatus(status).
		Save(context.Background())
	if err != nil {
		t.Fatalf("create question fixture: %v", err)
	}
	return entity
}

func QuestionCount(t *testing.T, client *ent.Client, schoolID uuid.UUID) int {
	t.Helper()
	count, err := client.Question.Query().Where(entquestion.SchoolIDEQ(schoolID)).Count(context.Background())
	if err != nil {
		t.Fatalf("count question fixtures: %v", err)
	}
	return count
}

func TeacherPrincipal(_ *ent.Client) auth.Principal {
	return auth.Principal{UserID: TeacherA, SchoolID: SchoolA, Role: auth.RoleTeacher, Subject: "数学", SessionVersion: 1}
}

func AdminPrincipal(_ *ent.Client) auth.Principal {
	return auth.Principal{UserID: AdminA, SchoolID: SchoolA, Role: auth.RoleAdmin, SessionVersion: 1}
}

func TeacherBPrincipal() auth.Principal {
	return auth.Principal{UserID: TeacherB, SchoolID: SchoolB, Role: auth.RoleTeacher, Subject: "数学", SessionVersion: 1}
}
