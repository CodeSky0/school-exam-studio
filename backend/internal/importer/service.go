package importer

import (
	"context"
	"io"
	"sort"

	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/question"
)

type Service struct {
	repository *Repository
	sanitizer  *question.Sanitizer
}

func NewService(repository *Repository, sanitizer *question.Sanitizer) *Service {
	if sanitizer == nil {
		sanitizer = question.NewSanitizer()
	}
	return &Service{repository: repository, sanitizer: sanitizer}
}

func (s *Service) Validate(ctx context.Context, principal auth.Principal, reader io.Reader) (Result, error) {
	result, _, err := s.parseAndValidate(ctx, principal, reader)
	return result, err
}

func (s *Service) ImportCSV(ctx context.Context, principal auth.Principal, reader io.Reader) (Result, error) {
	result, rows, err := s.parseAndValidate(ctx, principal, reader)
	if err != nil || len(result.Errors) > 0 {
		return result, err
	}
	imported, err := s.repository.Import(ctx, principal, rows)
	if err != nil {
		return Result{}, err
	}
	result.ValidCount = imported
	return result, nil
}

func (s *Service) Import(ctx context.Context, principal auth.Principal, rows []ImportRow) (int, error) {
	creator, err := s.repository.Creator(ctx, principal)
	if err != nil {
		return 0, err
	}
	normalized := make([]ImportRow, 0, len(rows))
	rowErrors := make([]RowError, 0)
	for index, row := range rows {
		line := row.Line
		if line < 2 {
			line = index + 2
		}
		if principal.Role == auth.RoleTeacher && row.Subject != creator.Subject {
			rowErrors = append(rowErrors, RowError{Line: line, Field: "subject", Message: "教师导入学科必须与账户学科一致"})
			continue
		}
		input, normalizeErr := s.sanitizer.Normalize(row.QuestionInput())
		if normalizeErr != nil {
			rowErrors = append(rowErrors, rowErrorFromValidation(line, normalizeErr))
			continue
		}
		normalized = append(normalized, importRowFromInput(line, input))
	}
	if len(rowErrors) > 0 {
		return 0, &RowValidationError{RowErrors: rowErrors}
	}
	return s.repository.Import(ctx, principal, normalized)
}

func (s *Service) parseAndValidate(ctx context.Context, principal auth.Principal, reader io.Reader) (Result, []ImportRow, error) {
	creator, err := s.repository.Creator(ctx, principal)
	if err != nil {
		return emptyResult(), nil, err
	}
	parsedRows, parseErrors, err := parseCSV(reader, s.sanitizer)
	if err != nil {
		return emptyResult(), nil, err
	}
	validRows := make([]ImportRow, 0, len(parsedRows))
	for _, row := range parsedRows {
		if principal.Role == auth.RoleTeacher && row.Subject != creator.Subject {
			parseErrors = append(parseErrors, RowError{Line: row.Line, Field: "subject", Message: "教师导入学科必须与账户学科一致"})
			continue
		}
		validRows = append(validRows, row)
	}
	sort.SliceStable(parseErrors, func(i, j int) bool { return parseErrors[i].Line < parseErrors[j].Line })
	result := Result{
		ValidCount: len(validRows),
		Preview:    make([]ImportRow, 0, min(len(validRows), PreviewRows)),
		Errors:     parseErrors,
	}
	for index := 0; index < len(validRows) && index < PreviewRows; index++ {
		result.Preview = append(result.Preview, validRows[index])
	}
	return result, validRows, nil
}

func rowErrorFromValidation(line int, err error) RowError {
	if validation, ok := err.(*question.ValidationError); ok {
		return RowError{Line: line, Field: validation.Violation.Field, Message: validation.Violation.Message}
	}
	return RowError{Line: line, Field: "row", Message: "题目内容未通过校验"}
}

func emptyResult() Result {
	return Result{Preview: []ImportRow{}, Errors: []RowError{}}
}
