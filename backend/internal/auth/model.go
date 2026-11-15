package auth

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"

	StatusActive   = "active"
	StatusDisabled = "disabled"
)

var (
	ErrValidation         = errors.New("validation failed")
	ErrAlreadyInitialized = errors.New("school already initialized")
	ErrInvalidSchoolCode  = errors.New("invalid school code")
	ErrEmailAlreadyExists = errors.New("email already exists in school")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionInvalid     = errors.New("session is no longer valid")
	ErrOldPassword        = errors.New("old password is incorrect")
)

var schoolCodePattern = regexp.MustCompile(`^[0-9]{8}$`)

type Principal struct {
	UserID         uuid.UUID
	SchoolID       uuid.UUID
	Role           string
	Subject        string
	SessionVersion int
}

type InitializeInput struct {
	SchoolName string
	AdminName  string
	Email      string
	Password   string
}

type RegisterInput struct {
	SchoolCode string
	Name       string
	Subject    string
	Email      string
	Password   string
}

type LoginInput struct {
	SchoolCode string
	Email      string
	Password   string
}

type ChangePasswordInput struct {
	OldPassword string
	NewPassword string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	SchoolID  uuid.UUID `json:"school_id"`
	Role      string    `json:"role"`
	Name      string    `json:"name"`
	Subject   string    `json:"subject"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type InitializeResult struct {
	SchoolID   uuid.UUID `json:"school_id"`
	SchoolCode string    `json:"school_code"`
	User       User      `json:"user"`
}

type LoginResult struct {
	Token string `json:"-"`
	User  User   `json:"user"`
}

type FieldViolation struct {
	Field   string
	Message string
}

type ValidationError struct {
	Violation FieldViolation
}

func (e *ValidationError) Error() string { return ErrValidation.Error() }
func (e *ValidationError) Unwrap() error { return ErrValidation }

func validationError(field, message string) error {
	return &ValidationError{Violation: FieldViolation{Field: field, Message: message}}
}

func normalizeInitializeInput(input InitializeInput) (InitializeInput, error) {
	var err error
	if input.SchoolName, err = normalizeText(input.SchoolName, "school_name", 2, 200); err != nil {
		return InitializeInput{}, err
	}
	if input.AdminName, err = normalizeText(input.AdminName, "admin_name", 2, 50); err != nil {
		return InitializeInput{}, err
	}
	if input.Email, err = normalizeEmail(input.Email); err != nil {
		return InitializeInput{}, err
	}
	if err := validatePassword(input.Password, "password"); err != nil {
		return InitializeInput{}, err
	}
	return input, nil
}

func normalizeRegisterInput(input RegisterInput) (RegisterInput, error) {
	var err error
	input.SchoolCode = strings.TrimSpace(input.SchoolCode)
	if !schoolCodePattern.MatchString(input.SchoolCode) {
		return RegisterInput{}, validationError("school_code", "学校代码必须为8位数字")
	}
	if input.Name, err = normalizeText(input.Name, "name", 2, 50); err != nil {
		return RegisterInput{}, err
	}
	if input.Subject, err = normalizeText(input.Subject, "subject", 1, 30); err != nil {
		return RegisterInput{}, err
	}
	if input.Email, err = normalizeEmail(input.Email); err != nil {
		return RegisterInput{}, err
	}
	if err := validatePassword(input.Password, "password"); err != nil {
		return RegisterInput{}, err
	}
	return input, nil
}

func normalizeText(value, field string, minLength, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", validationError(field, "字段包含无效字符")
	}
	length := utf8.RuneCountInString(value)
	if length < minLength || length > maxLength {
		return "", validationError(field, fmt.Sprintf("长度必须为%d-%d个字符", minLength, maxLength))
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", validationError(field, "字段不能包含控制字符")
		}
	}
	return value, nil
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 320 || strings.Count(value, "@") != 1 {
		return "", validationError("email", "邮箱格式不正确")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return "", validationError("email", "邮箱格式不正确")
	}
	return value, nil
}

func validatePassword(password, field string) error {
	if !utf8.ValidString(password) {
		return validationError(field, "密码包含无效字符")
	}
	length := utf8.RuneCountInString(password)
	if length < 10 || length > 72 || len(password) > 72 {
		return validationError(field, "密码必须为10-72个字符，且UTF-8编码不超过72字节")
	}
	return nil
}

func validRole(role string) bool {
	return role == RoleAdmin || role == RoleTeacher
}
