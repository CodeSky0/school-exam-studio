package paper

import (
	"context"
	"math"
	"strings"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, principal auth.Principal, input Input) (Paper, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return Paper{}, err
	}
	return s.repository.create(ctx, principal, normalized)
}

func (s *Service) Update(ctx context.Context, principal auth.Principal, id uuid.UUID, input Input) (Paper, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return Paper{}, err
	}
	return s.repository.update(ctx, principal, id, normalized)
}

func (s *Service) Get(ctx context.Context, principal auth.Principal, id uuid.UUID) (Paper, error) {
	return s.repository.Get(ctx, principal, id)
}

func (s *Service) List(ctx context.Context, principal auth.Principal, filter Filter) (ListResult, error) {
	filter.Subject = strings.TrimSpace(filter.Subject)
	filter.Grade = strings.TrimSpace(filter.Grade)
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	if int64(filter.Page-1) > math.MaxInt64/int64(filter.PageSize) {
		return ListResult{}, validationError("page", "分页参数过大")
	}
	return s.repository.List(ctx, principal, filter)
}

func (s *Service) Delete(ctx context.Context, principal auth.Principal, id uuid.UUID) error {
	return s.repository.Delete(ctx, principal, id)
}
