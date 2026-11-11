package admin

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

var (
	ErrValidation = errors.New("admin validation failed")
	ErrNotFound   = errors.New("admin resource not found")
	ErrForbidden  = errors.New("admin operation forbidden")
)

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

type School struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	SchoolCode string    `json:"school_code"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TeacherFilter struct {
	Subject  string
	Status   string
	Page     int
	PageSize int
}

type TeacherListResult struct {
	Items    []auth.User `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

type TeacherStatusStats struct {
	Active   int `json:"active"`
	Disabled int `json:"disabled"`
}

type QuestionStats struct {
	Draft     int `json:"draft"`
	Published int `json:"published"`
	Archived  int `json:"archived"`
}

type Stats struct {
	Teachers  TeacherStatusStats `json:"teachers"`
	Questions QuestionStats      `json:"questions"`
	Papers    int                `json:"papers"`
}

type TeacherStats struct {
	Questions QuestionStats `json:"questions"`
	Papers    int           `json:"papers"`
}
