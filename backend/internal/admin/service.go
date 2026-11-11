package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

const temporaryPasswordBytes = 24

type Service struct {
	repository *Repository
	hasher     *auth.PasswordHasher
}

func NewService(repository *Repository, hasher *auth.PasswordHasher) *Service {
	if hasher == nil {
		hasher = auth.NewPasswordHasher(0)
	}
	return &Service{repository: repository, hasher: hasher}
}

func (s *Service) ListTeachers(ctx context.Context, principal auth.Principal, filter TeacherFilter) (TeacherListResult, error) {
	if !validAdmin(principal) {
		return TeacherListResult{}, ErrForbidden
	}
	filter.Subject = strings.TrimSpace(filter.Subject)
	filter.Status = strings.TrimSpace(filter.Status)
	if utf8.RuneCountInString(filter.Subject) > 30 {
		return TeacherListResult{}, validationError("subject", "学科筛选不能超过30个字符")
	}
	if filter.Status != "" && filter.Status != auth.StatusActive && filter.Status != auth.StatusDisabled {
		return TeacherListResult{}, validationError("status", "教师状态筛选只能为active或disabled")
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
	if int64(filter.Page-1) > math.MaxInt64/int64(filter.PageSize) {
		return TeacherListResult{}, validationError("page", "分页参数过大")
	}
	return s.repository.ListTeachers(ctx, principal, filter)
}

func (s *Service) UpdateTeacherStatus(ctx context.Context, principal auth.Principal, teacherID uuid.UUID, status string) (auth.User, error) {
	if !validAdmin(principal) {
		return auth.User{}, ErrForbidden
	}
	if teacherID == uuid.Nil {
		return auth.User{}, validationError("id", "教师ID不能为空")
	}
	if status != auth.StatusActive && status != auth.StatusDisabled {
		return auth.User{}, validationError("status", "教师状态只能为active或disabled")
	}
	return s.repository.UpdateTeacherStatus(ctx, principal, teacherID, status)
}

func (s *Service) ResetTeacherPassword(ctx context.Context, principal auth.Principal, teacherID uuid.UUID) (string, error) {
	if !validAdmin(principal) {
		return "", ErrForbidden
	}
	if teacherID == uuid.Nil {
		return "", validationError("id", "教师ID不能为空")
	}
	random := make([]byte, temporaryPasswordBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	temporaryPassword := base64.RawURLEncoding.EncodeToString(random)
	passwordHash, err := s.hasher.Hash(temporaryPassword)
	if err != nil {
		return "", err
	}
	if err := s.repository.ResetTeacherPassword(ctx, principal, teacherID, passwordHash); err != nil {
		return "", err
	}
	return temporaryPassword, nil
}

func (s *Service) GetSchool(ctx context.Context, principal auth.Principal) (School, error) {
	if !validAdmin(principal) {
		return School{}, ErrForbidden
	}
	return s.repository.GetSchool(ctx, principal)
}

func (s *Service) UpdateSchool(ctx context.Context, principal auth.Principal, name string) (School, error) {
	if !validAdmin(principal) {
		return School{}, ErrForbidden
	}
	name = strings.TrimSpace(name)
	length := utf8.RuneCountInString(name)
	if !utf8.ValidString(name) || length < 1 || length > 200 {
		return School{}, validationError("name", "学校名称长度必须为1-200个字符")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return School{}, validationError("name", "学校名称不能包含控制字符")
		}
	}
	return s.repository.UpdateSchool(ctx, principal, name)
}

func (s *Service) Stats(ctx context.Context, principal auth.Principal) (Stats, error) {
	if !validAdmin(principal) {
		return Stats{}, ErrForbidden
	}
	return s.repository.Stats(ctx, principal)
}

func (s *Service) TeacherStats(ctx context.Context, principal auth.Principal) (TeacherStats, error) {
	if principal.Role != auth.RoleTeacher {
		return TeacherStats{}, ErrForbidden
	}
	return s.repository.TeacherStats(ctx, principal)
}
