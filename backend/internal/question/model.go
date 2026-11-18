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

	MaxBatchQuestions = 100
)

type Option struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

// BatchFailure 描述批量操作中某一条目的失败原因，其余条目仍会继续执行。
type BatchFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// BatchStatusResult 是批量发布/归档的结果，成功与失败条目都会返回。
type BatchStatusResult struct {
	Status    string         `json:"status"`
	Total     int            `json:"total"`
	Succeeded []string       `json:"succeeded"`
	Failed    []BatchFailure `json:"failed"`
}

// BatchDeleteResult 是批量删除的结果，成功与失败条目都会返回。
type BatchDeleteResult struct {
	Total   int            `json:"total"`
	Deleted []string       `json:"deleted"`
	Failed  []BatchFailure `json:"failed"`
}

type Question struct {
	ID               uuid.UUID      `json:"id"`
	CreatorID        uuid.UUID      `json:"creator_id"`
	CreatorName      string         `json:"creator_name"`
	Subject          string         `json:"subject"`
	Grade            string         `json:"grade"`
	QuestionType     string         `json:"question_type"`
	Difficulty       int            `json:"difficulty"`
	Tags             []string       `json:"tags"`
	Category         string         `json:"category"`
	KnowledgePoints  []string       `json:"knowledge_points"`
	Content          string         `json:"content"`
	Options          []Option       `json:"options"`
	Answer           string         `json:"answer"`
	Explanation      string         `json:"explanation"`
	Status           string         `json:"status"`
	AIGenerated      bool           `json:"ai_generated"`
	AIMeta           map[string]any `json:"ai_meta"`
	ContentHash      string         `json:"content_hash"`
	SourceQuestionID *uuid.UUID     `json:"source_question_id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type Input struct {
	Subject          string         `json:"subject"`
	Grade            string         `json:"grade"`
	QuestionType     string         `json:"question_type"`
	Difficulty       int            `json:"difficulty"`
	Tags             []string       `json:"tags"`
	Category         string         `json:"category"`
	KnowledgePoints  []string       `json:"knowledge_points"`
	Content          string         `json:"content"`
	Options          []Option       `json:"options"`
	Answer           string         `json:"answer"`
	Explanation      string         `json:"explanation"`
	SourceQuestionID uuid.UUID      `json:"-"`
	AIGenerated      bool           `json:"-"`
	AIMeta           map[string]any `json:"-"`
}

type CreateInput = Input
type UpdateInput = Input

type QuestionFilter struct {
	Subject        string
	Grade          string
	Type           string
	Tag            string
	Category       string
	KnowledgePoint string
	CreatorID      string
	Status         string
	Difficulty     int
	Page           int
	PageSize       int
}

// AIApplyPatch 描述 AI 对一道草稿的受控修改，只有显式提供的字段会被更新。
type AIApplyPatch struct {
	Category        *string
	KnowledgePoints *[]string
	Tags            *[]string
	Difficulty      *int
	Answer          *string
	Explanation     *string
	Provider        string
	Model           string
	Confidence      float64
	Rationale       string
	ReviewRequired  bool
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
