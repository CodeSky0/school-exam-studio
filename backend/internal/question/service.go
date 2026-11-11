package question

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

type Service struct {
	repository *Repository
	sanitizer  *Sanitizer
}

func NewService(repository *Repository, sanitizers ...*Sanitizer) *Service {
	sanitizer := NewSanitizer()
	if len(sanitizers) > 0 && sanitizers[0] != nil {
		sanitizer = sanitizers[0]
	}
	return &Service{repository: repository, sanitizer: sanitizer}
}

func (s *Service) Create(ctx context.Context, principal auth.Principal, input Input) (Question, error) {
	normalized, err := s.sanitizer.Normalize(input)
	if err != nil {
		return Question{}, err
	}
	return s.repository.Create(ctx, principal, normalized)
}

func (s *Service) Update(ctx context.Context, principal auth.Principal, id uuid.UUID, input Input) (Question, error) {
	normalized, err := s.sanitizer.Normalize(input)
	if err != nil {
		return Question{}, err
	}
	return s.repository.Update(ctx, principal, id, normalized)
}

func (s *Service) Get(ctx context.Context, principal auth.Principal, id uuid.UUID) (Question, error) {
	return s.repository.Get(ctx, principal, id)
}

func (s *Service) List(ctx context.Context, principal auth.Principal, filter QuestionFilter) (ListResult, error) {
	filter.Subject = strings.TrimSpace(filter.Subject)
	filter.Grade = strings.TrimSpace(filter.Grade)
	filter.Type = strings.TrimSpace(filter.Type)
	filter.Tag = strings.TrimSpace(filter.Tag)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Type != "" && !ValidType(filter.Type) {
		return ListResult{}, validationError("question_type", "题型筛选值不受支持")
	}
	if filter.Status != "" && !ValidStatus(filter.Status) {
		return ListResult{}, validationError("status", "题目状态筛选值不受支持")
	}
	if filter.Difficulty < 0 || filter.Difficulty > 5 {
		return ListResult{}, validationError("difficulty", "难度筛选值必须为1-5")
	}
	if filter.CreatorID != "" {
		if _, err := uuid.Parse(filter.CreatorID); err != nil {
			return ListResult{}, validationError("creator_id", "创建人ID格式不正确")
		}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return s.repository.List(ctx, principal, filter)
}

func (s *Service) Delete(ctx context.Context, principal auth.Principal, id uuid.UUID) error {
	return s.repository.Delete(ctx, principal, id)
}

func (s *Service) ResolvePublished(ctx context.Context, principal auth.Principal, ids []uuid.UUID) ([]Question, error) {
	return s.repository.ResolvePublished(ctx, principal, ids)
}

func (s *Service) UpdateStatus(ctx context.Context, principal auth.Principal, id uuid.UUID, status string) (Question, error) {
	if status != StatusPublished && status != StatusArchived {
		return Question{}, validationError("status", "审核状态只能为published或archived")
	}
	return s.repository.UpdateStatus(ctx, principal, id, status)
}
