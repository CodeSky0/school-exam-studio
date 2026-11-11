package paper

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/question"
)

var (
	ErrValidation          = errors.New("paper validation failed")
	ErrNotFound            = errors.New("paper not found")
	ErrForbidden           = errors.New("paper operation forbidden")
	ErrQuestionUnavailable = errors.New("paper question unavailable")
	ErrQuestionMismatch    = errors.New("paper question subject or grade mismatch")
	ErrDuplicateQuestion   = errors.New("paper question duplicated")
)

type Paper struct {
	ID              uuid.UUID       `json:"id"`
	CreatorID       uuid.UUID       `json:"creator_id"`
	SchoolName      string          `json:"school_name"`
	Title           string          `json:"title"`
	Subject         string          `json:"subject"`
	Grade           string          `json:"grade"`
	TotalScore      string          `json:"total_score"`
	DurationMinutes int             `json:"duration_minutes"`
	Questions       []PaperQuestion `json:"questions,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type PaperQuestion struct {
	QuestionID    uuid.UUID         `json:"question_id"`
	OrderIndex    int               `json:"order_index"`
	AssignedScore string            `json:"assigned_score"`
	Question      question.Question `json:"question"`
}

type QuestionInput struct {
	QuestionID    uuid.UUID `json:"question_id"`
	AssignedScore string    `json:"assigned_score"`
}

type Input struct {
	Title           string          `json:"title"`
	Subject         string          `json:"subject"`
	Grade           string          `json:"grade"`
	DurationMinutes int             `json:"duration_minutes"`
	Questions       []QuestionInput `json:"questions"`
}

type CreateInput = Input
type UpdateInput = Input
type PaperQuestionInput = QuestionInput

type PaperFilter struct {
	Subject  string
	Grade    string
	Page     int
	PageSize int
}

type Filter = PaperFilter

type ListResult struct {
	Items    []Paper `json:"items"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
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

type normalizedQuestion struct {
	questionID uuid.UUID
	score      Score
}

type normalizedInput struct {
	title           string
	subject         string
	grade           string
	durationMinutes int
	questions       []normalizedQuestion
}

func normalizeInput(input Input) (normalizedInput, error) {
	normalized := normalizedInput{durationMinutes: input.DurationMinutes}
	var err error
	if normalized.title, err = normalizeText(input.Title, "title", 1, 200); err != nil {
		return normalizedInput{}, err
	}
	if normalized.subject, err = normalizeText(input.Subject, "subject", 1, 30); err != nil {
		return normalizedInput{}, err
	}
	if normalized.grade, err = normalizeText(input.Grade, "grade", 1, 30); err != nil {
		return normalizedInput{}, err
	}
	if input.DurationMinutes < 1 || input.DurationMinutes > 600 {
		return normalizedInput{}, validationError("duration_minutes", "考试时长必须为1-600分钟")
	}
	if len(input.Questions) == 0 {
		return normalizedInput{}, validationError("questions", "试卷至少需要一道题")
	}
	normalized.questions = make([]normalizedQuestion, 0, len(input.Questions))
	seen := make(map[uuid.UUID]struct{}, len(input.Questions))
	for index, item := range input.Questions {
		if item.QuestionID == uuid.Nil {
			return normalizedInput{}, validationError(fmt.Sprintf("questions[%d].question_id", index), "题目ID不能为空")
		}
		if _, exists := seen[item.QuestionID]; exists {
			return normalizedInput{}, ErrDuplicateQuestion
		}
		seen[item.QuestionID] = struct{}{}
		score, err := ParseScore(item.AssignedScore)
		if err != nil || !score.isPositive() {
			return normalizedInput{}, validationError(fmt.Sprintf("questions[%d].assigned_score", index), "分值必须大于0且最多两位小数")
		}
		if score.exceedsMaximum() {
			return normalizedInput{}, validationError(fmt.Sprintf("questions[%d].assigned_score", index), "单题分值不能超过999999.99")
		}
		normalized.questions = append(normalized.questions, normalizedQuestion{questionID: item.QuestionID, score: score})
	}
	return normalized, nil
}

func normalizeText(value, field string, minimum, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", validationError(field, "字段包含无效字符")
	}
	length := utf8.RuneCountInString(value)
	if length < minimum || length > maximum {
		return "", validationError(field, fmt.Sprintf("长度必须为%d-%d个字符", minimum, maximum))
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", validationError(field, "字段不能包含控制字符")
		}
	}
	return value, nil
}

func validPrincipal(principal auth.Principal) bool {
	return principal.UserID != uuid.Nil && principal.SchoolID != uuid.Nil &&
		(principal.Role == auth.RoleTeacher || principal.Role == auth.RoleAdmin)
}
