package question

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"

	TypeSingleChoice   = "single_choice"
	TypeMultipleChoice = "multiple_choice"
	TypeTrueFalse      = "true_false"
	TypeFillBlank      = "fill_blank"
	TypeEssay          = "essay"
)

var (
	ErrValidation        = errors.New("question validation failed")
	ErrNotFound          = errors.New("question not found")
	ErrForbidden         = errors.New("question operation forbidden")
	ErrSubjectMismatch   = errors.New("teacher subject does not match question subject")
	ErrInvalidTransition = errors.New("invalid question status transition")
)

type Option struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type Question struct {
	ID           uuid.UUID `json:"id"`
	CreatorID    uuid.UUID `json:"creator_id"`
	CreatorName  string    `json:"creator_name"`
	Subject      string    `json:"subject"`
	Grade        string    `json:"grade"`
	QuestionType string    `json:"question_type"`
	Difficulty   int       `json:"difficulty"`
	Tags         []string  `json:"tags"`
	Content      string    `json:"content"`
	Options      []Option  `json:"options"`
	Answer       string    `json:"answer"`
	Explanation  string    `json:"explanation"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Input struct {
	Subject      string   `json:"subject"`
	Grade        string   `json:"grade"`
	QuestionType string   `json:"question_type"`
	Difficulty   int      `json:"difficulty"`
	Tags         []string `json:"tags"`
	Content      string   `json:"content"`
	Options      []Option `json:"options"`
	Answer       string   `json:"answer"`
	Explanation  string   `json:"explanation"`
}

type CreateInput = Input
type UpdateInput = Input

type QuestionFilter struct {
	Subject    string
	Grade      string
	Type       string
	Tag        string
	CreatorID  string
	Status     string
	Difficulty int
	Page       int
	PageSize   int
}

type ListResult struct {
	Items    []Question `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
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

func ValidType(value string) bool {
	switch value {
	case TypeSingleChoice, TypeMultipleChoice, TypeTrueFalse, TypeFillBlank, TypeEssay:
		return true
	default:
		return false
	}
}

func ValidStatus(value string) bool {
	return value == StatusDraft || value == StatusPublished || value == StatusArchived
}
