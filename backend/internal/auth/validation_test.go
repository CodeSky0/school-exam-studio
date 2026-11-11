package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"school-exam-studio/backend/internal/auth"
)

func TestInputValidation(t *testing.T) {
	service := auth.NewService(nil, auth.NewPasswordHasher(bcrypt.MinCost), testSecret, time.Hour)
	validInitialize := auth.InitializeInput{
		SchoolName: "实验中学",
		AdminName:  "管理员",
		Email:      "admin@example.com",
		Password:   "CorrectHorse123!",
	}
	validRegister := auth.RegisterInput{
		SchoolCode: "12345678",
		Name:       "数学教师",
		Subject:    "数学",
		Email:      "teacher@example.com",
		Password:   "CorrectHorse123!",
	}
	tests := []struct {
		name  string
		field string
		call  func() error
	}{
		{"school name empty", "school_name", func() error {
			input := validInitialize
			input.SchoolName = " "
			_, err := service.Initialize(context.Background(), input)
			return err
		}},
		{"school name too long", "school_name", func() error {
			input := validInitialize
			input.SchoolName = strings.Repeat("校", 201)
			_, err := service.Initialize(context.Background(), input)
			return err
		}},
		{"admin name too short", "admin_name", func() error {
			input := validInitialize
			input.AdminName = "管"
			_, err := service.Initialize(context.Background(), input)
			return err
		}},
		{"admin name too long", "admin_name", func() error {
			input := validInitialize
			input.AdminName = strings.Repeat("师", 51)
			_, err := service.Initialize(context.Background(), input)
			return err
		}},
		{"school code not eight digits", "school_code", func() error {
			input := validRegister
			input.SchoolCode = "1234567a"
			_, err := service.Register(context.Background(), input)
			return err
		}},
		{"teacher name too short", "name", func() error {
			input := validRegister
			input.Name = "师"
			_, err := service.Register(context.Background(), input)
			return err
		}},
		{"subject empty", "subject", func() error {
			input := validRegister
			input.Subject = " "
			_, err := service.Register(context.Background(), input)
			return err
		}},
		{"subject too long", "subject", func() error {
			input := validRegister
			input.Subject = strings.Repeat("数", 31)
			_, err := service.Register(context.Background(), input)
			return err
		}},
		{"email invalid", "email", func() error {
			input := validRegister
			input.Email = "not-an-email"
			_, err := service.Register(context.Background(), input)
			return err
		}},
		{"password too short", "password", func() error {
			input := validRegister
			input.Password = "Short123!"
			_, err := service.Register(context.Background(), input)
			return err
		}},
		{"password too long", "password", func() error {
			input := validRegister
			input.Password = strings.Repeat("a", 73)
			_, err := service.Register(context.Background(), input)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if !errors.Is(err, auth.ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
			var validation *auth.ValidationError
			if !errors.As(err, &validation) || validation.Violation.Field != test.field {
				t.Fatalf("unexpected validation detail: %+v", validation)
			}
		})
	}
}

func TestLoginValidationUsesGenericCredentialsError(t *testing.T) {
	service := auth.NewService(nil, auth.NewPasswordHasher(bcrypt.MinCost), testSecret, time.Hour)
	inputs := []auth.LoginInput{
		{Email: "", Password: "CorrectHorse123!"},
		{Email: "bad", Password: "CorrectHorse123!"},
		{Email: "teacher@example.com", Password: "short"},
	}
	for _, input := range inputs {
		if _, err := service.Login(context.Background(), input); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("expected generic credentials error, got %v", err)
		}
	}
}
