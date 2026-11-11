package auth_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/user"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/platformtest"
)

const (
	testSecret       = "01234567890123456789012345678901"
	initialPassword  = "CorrectHorse123!"
	changedPassword  = "NewCorrectHorse456!"
	testSchoolName   = "实验中学"
	testAdminEmail   = "admin@example.com"
	testTeacherEmail = "teacher@example.com"
)

func newIntegrationService(client *ent.Client) *auth.Service {
	return auth.NewService(
		auth.NewRepository(client),
		auth.NewPasswordHasher(bcrypt.MinCost),
		testSecret,
		2*time.Hour,
	)
}

func initializeSchool(t *testing.T, service *auth.Service) auth.InitializeResult {
	t.Helper()
	result, err := service.Initialize(context.Background(), auth.InitializeInput{
		SchoolName: "  " + testSchoolName + "  ",
		AdminName:  "管理员",
		Email:      "  ADMIN@EXAMPLE.COM ",
		Password:   initialPassword,
	})
	if err != nil {
		t.Fatalf("initialize school: %v", err)
	}
	return result
}

func registerTeacher(t *testing.T, service *auth.Service, schoolCode string) auth.User {
	t.Helper()
	teacher, err := service.Register(context.Background(), auth.RegisterInput{
		SchoolCode: schoolCode,
		Name:       "数学教师",
		Subject:    "数学",
		Email:      " TEACHER@EXAMPLE.COM ",
		Password:   initialPassword,
	})
	if err != nil {
		t.Fatalf("register teacher: %v", err)
	}
	return teacher
}

func TestInitializeRegisterLoginAndChangePassword(t *testing.T) {
	client := platformtest.Open(t)
	service := newIntegrationService(client)
	ctx := context.Background()
	initialized, err := service.InitializationStatus(ctx)
	if err != nil || initialized {
		t.Fatalf("unexpected initial status: initialized=%v err=%v", initialized, err)
	}
	result := initializeSchool(t, service)
	if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(result.SchoolCode) {
		t.Fatalf("invalid school code %q", result.SchoolCode)
	}
	if result.User.Email != testAdminEmail || result.User.Role != auth.RoleAdmin || result.User.Status != auth.StatusActive {
		t.Fatalf("unexpected administrator: %+v", result.User)
	}
	if _, err := service.Initialize(ctx, auth.InitializeInput{
		SchoolName: testSchoolName,
		AdminName:  "管理员",
		Email:      testAdminEmail,
		Password:   initialPassword,
	}); !errors.Is(err, auth.ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized, got %v", err)
	}
	initialized, err = service.InitializationStatus(ctx)
	if err != nil || !initialized {
		t.Fatalf("unexpected initialized status: initialized=%v err=%v", initialized, err)
	}

	teacher := registerTeacher(t, service, result.SchoolCode)
	if teacher.Email != testTeacherEmail || teacher.Role != auth.RoleTeacher || teacher.Status != auth.StatusActive {
		t.Fatalf("unexpected teacher: %+v", teacher)
	}
	if _, err := service.Register(ctx, auth.RegisterInput{
		SchoolCode: result.SchoolCode,
		Name:       "另一位教师",
		Subject:    "数学",
		Email:      "TEACHER@example.com",
		Password:   initialPassword,
	}); !errors.Is(err, auth.ErrEmailAlreadyExists) {
		t.Fatalf("expected school-local email conflict, got %v", err)
	}

	invalidLogins := []auth.LoginInput{
		{Email: "missing@example.com", Password: initialPassword},
		{Email: testTeacherEmail, Password: "WrongPassword123!"},
	}
	for _, input := range invalidLogins {
		if _, err := service.Login(ctx, input); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("expected generic login error, got %v", err)
		}
	}
	login, err := service.Login(ctx, auth.LoginInput{
		Email:    " TEACHER@EXAMPLE.COM ",
		Password: initialPassword,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	principal, err := service.Tokens().Parse(login.Token)
	if err != nil || principal.UserID != teacher.ID || principal.SchoolID != result.SchoolID {
		t.Fatalf("unexpected token principal: %+v %v", principal, err)
	}
	if _, err := service.Authenticate(ctx, principal); err != nil {
		t.Fatalf("authenticate current principal: %v", err)
	}
	if err := service.ChangePassword(ctx, principal, auth.ChangePasswordInput{
		OldPassword: "WrongPassword123!",
		NewPassword: changedPassword,
	}); !errors.Is(err, auth.ErrOldPassword) {
		t.Fatalf("expected old password error, got %v", err)
	}
	if err := service.ChangePassword(ctx, principal, auth.ChangePasswordInput{
		OldPassword: initialPassword,
		NewPassword: changedPassword,
	}); err != nil {
		t.Fatalf("change password: %v", err)
	}
	stored, err := client.User.Query().Where(
		user.IDEQ(teacher.ID),
		user.SchoolIDEQ(result.SchoolID),
	).Only(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SessionVersion != 2 {
		t.Fatalf("session version = %d, want 2", stored.SessionVersion)
	}
	if _, err := service.Authenticate(ctx, principal); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("expected old session rejection, got %v", err)
	}
	if _, err := service.Login(ctx, auth.LoginInput{
		SchoolCode: result.SchoolCode,
		Email:      testTeacherEmail,
		Password:   initialPassword,
	}); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected old password rejection, got %v", err)
	}
	if _, err := service.Login(ctx, auth.LoginInput{
		SchoolCode: result.SchoolCode,
		Email:      testTeacherEmail,
		Password:   changedPassword,
	}); err != nil {
		t.Fatalf("login with changed password: %v", err)
	}
}

func TestConcurrentInitializeOnlySucceedsOnce(t *testing.T) {
	client := platformtest.Open(t)
	service := newIntegrationService(client)
	const attempts = 8
	var waitGroup sync.WaitGroup
	errorsByAttempt := make(chan error, attempts)
	successes := make(chan auth.InitializeResult, attempts)
	for index := range attempts {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result, err := service.Initialize(context.Background(), auth.InitializeInput{
				SchoolName: fmt.Sprintf("实验中学%d", index),
				AdminName:  "管理员",
				Email:      fmt.Sprintf("admin%d@example.com", index),
				Password:   initialPassword,
			})
			if err == nil {
				successes <- result
				return
			}
			errorsByAttempt <- err
		}()
	}
	waitGroup.Wait()
	close(successes)
	close(errorsByAttempt)
	if len(successes) != 1 {
		t.Fatalf("successful initializations = %d, want 1", len(successes))
	}
	for err := range errorsByAttempt {
		if !errors.Is(err, auth.ErrAlreadyInitialized) {
			t.Fatalf("unexpected concurrent initialization error: %v", err)
		}
	}
	if count, err := client.School.Query().Count(context.Background()); err != nil || count != 1 {
		t.Fatalf("school count = %d, want 1 (err=%v)", count, err)
	}
	if count, err := client.User.Query().Count(context.Background()); err != nil || count != 1 {
		t.Fatalf("user count = %d, want 1 (err=%v)", count, err)
	}
}
