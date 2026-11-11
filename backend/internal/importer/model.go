package importer

import (
	"errors"
	"fmt"

	"school-exam-studio/backend/internal/question"
)

const (
	MaxFileSize = 5 << 20
	MaxDataRows = 1000
	PreviewRows = 50
)

var (
	ErrFileTooLarge = errors.New("CSV file exceeds 5 MiB")
	ErrForbidden    = errors.New("CSV import forbidden")
	ErrValidation   = errors.New("CSV validation failed")
)

type ImportRow struct {
	Line         int               `json:"line"`
	Type         string            `json:"question_type"`
	Grade        string            `json:"grade"`
	Subject      string            `json:"subject"`
	Difficulty   int               `json:"difficulty"`
	Tags         []string          `json:"tags"`
	Content      string            `json:"content"`
	Options      []question.Option `json:"options"`
	Answer       string            `json:"answer"`
	Explanation  string            `json:"explanation"`
}

func (r ImportRow) QuestionInput() question.Input {
	return question.Input{
		Subject:      r.Subject,
		Grade:        r.Grade,
		QuestionType: r.Type,
		Difficulty:   r.Difficulty,
		Tags:         r.Tags,
		Content:      r.Content,
		Options:      r.Options,
		Answer:       r.Answer,
		Explanation:  r.Explanation,
	}
}

func importRowFromInput(line int, input question.Input) ImportRow {
	return ImportRow{
		Line:        line,
		Type:        input.QuestionType,
		Grade:       input.Grade,
		Subject:     input.Subject,
		Difficulty:  input.Difficulty,
		Tags:        input.Tags,
		Content:     input.Content,
		Options:     input.Options,
		Answer:      input.Answer,
		Explanation: input.Explanation,
	}
}

type RowError struct {
	Line    int    `json:"line"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Result struct {
	ValidCount int         `json:"valid_count"`
	Preview    []ImportRow `json:"preview"`
	Errors     []RowError  `json:"errors"`
}

type RowValidationError struct {
	RowErrors []RowError
}

func (e *RowValidationError) Error() string {
	return fmt.Sprintf("CSV contains %d invalid rows", len(e.RowErrors))
}
func (e *RowValidationError) Unwrap() error { return ErrValidation }
