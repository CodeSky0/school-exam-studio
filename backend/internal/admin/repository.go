package admin

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	entpaper "school-exam-studio/backend/ent/paper"
	"school-exam-studio/backend/ent/predicate"
	entquestion "school-exam-studio/backend/ent/question"
	entschool "school-exam-studio/backend/ent/school"
	entuser "school-exam-studio/backend/ent/user"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/question"
)

type Repository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) ListTeachers(ctx context.Context, principal auth.Principal, filter TeacherFilter) (TeacherListResult, error) {
	if !validAdmin(principal) {
		return TeacherListResult{}, ErrForbidden
	}
	predicates := []predicate.User{
		entuser.SchoolIDEQ(principal.SchoolID),
		entuser.RoleEQ(auth.RoleTeacher),
		entuser.DeletedAtIsNil(),
	}
	if filter.Subject != "" {
		predicates = append(predicates, entuser.SubjectEQ(filter.Subject))
	}
	if filter.Status != "" {
		predicates = append(predicates, entuser.StatusEQ(filter.Status))
	}
	query := r.client.User.Query().Where(predicates...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return TeacherListResult{}, err
	}
	entities, err := query.Order(
		entuser.ByCreatedAt(entsql.OrderDesc()),
		entuser.ByID(entsql.OrderDesc()),
	).Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).All(ctx)
	if err != nil {
		return TeacherListResult{}, err
	}
	items := make([]auth.User, 0, len(entities))
	for _, entity := range entities {
		items = append(items, userFromEnt(entity))
	}
	return TeacherListResult{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (r *Repository) UpdateTeacherStatus(ctx context.Context, principal auth.Principal, teacherID uuid.UUID, status string) (auth.User, error) {
	if !validAdmin(principal) {
		return auth.User{}, ErrForbidden
	}
	updated, err := r.client.User.Update().Where(
		entuser.SchoolIDEQ(principal.SchoolID),
		entuser.IDEQ(teacherID),
		entuser.RoleEQ(auth.RoleTeacher),
		entuser.DeletedAtIsNil(),
		entuser.StatusNEQ(status),
	).SetStatus(status).AddSessionVersion(1).Save(ctx)
	if err != nil {
		return auth.User{}, err
	}
	if updated > 1 {
		return auth.User{}, ErrNotFound
	}
	entity, err := r.teacherByID(ctx, principal.SchoolID, teacherID)
	if err != nil {
		return auth.User{}, err
	}
	return userFromEnt(entity), nil
}

func (r *Repository) ResetTeacherPassword(ctx context.Context, principal auth.Principal, teacherID uuid.UUID, passwordHash string) error {
	if !validAdmin(principal) {
		return ErrForbidden
	}
	updated, err := r.client.User.Update().Where(
		entuser.SchoolIDEQ(principal.SchoolID),
		entuser.IDEQ(teacherID),
		entuser.RoleEQ(auth.RoleTeacher),
		entuser.DeletedAtIsNil(),
	).SetPasswordHash(passwordHash).AddSessionVersion(1).Save(ctx)
	if err != nil {
		return err
	}
	if updated != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetSchool(ctx context.Context, principal auth.Principal) (School, error) {
	if !validAdmin(principal) {
		return School{}, ErrForbidden
	}
	entity, err := r.client.School.Query().Where(
		entschool.IDEQ(principal.SchoolID),
		entschool.DeletedAtIsNil(),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return School{}, ErrNotFound
	}
	if err != nil {
		return School{}, err
	}
	return schoolFromEnt(entity), nil
}

func (r *Repository) UpdateSchool(ctx context.Context, principal auth.Principal, name string) (School, error) {
	if !validAdmin(principal) {
		return School{}, ErrForbidden
	}
	updated, err := r.client.School.Update().Where(
		entschool.IDEQ(principal.SchoolID),
		entschool.DeletedAtIsNil(),
	).SetName(name).Save(ctx)
	if err != nil {
		return School{}, err
	}
	if updated != 1 {
		return School{}, ErrNotFound
	}
	return r.GetSchool(ctx, principal)
}

func (r *Repository) Stats(ctx context.Context, principal auth.Principal) (Stats, error) {
	if !validAdmin(principal) {
		return Stats{}, ErrForbidden
	}
	active, err := r.countTeachers(ctx, principal.SchoolID, auth.StatusActive)
	if err != nil {
		return Stats{}, err
	}
	disabled, err := r.countTeachers(ctx, principal.SchoolID, auth.StatusDisabled)
	if err != nil {
		return Stats{}, err
	}
	questions, err := r.countQuestions(ctx, principal.SchoolID, uuid.Nil)
	if err != nil {
		return Stats{}, err
	}
	papers, err := r.countPapers(ctx, principal.SchoolID, uuid.Nil)
	if err != nil {
		return Stats{}, err
	}
	return Stats{
		Teachers:  TeacherStatusStats{Active: active, Disabled: disabled},
		Questions: questions,
		Papers:    papers,
	}, nil
}

func (r *Repository) TeacherStats(ctx context.Context, principal auth.Principal) (TeacherStats, error) {
	if principal.Role != auth.RoleTeacher || principal.UserID == uuid.Nil || principal.SchoolID == uuid.Nil {
		return TeacherStats{}, ErrForbidden
	}
	questions, err := r.countQuestions(ctx, principal.SchoolID, principal.UserID)
	if err != nil {
		return TeacherStats{}, err
	}
	papers, err := r.countPapers(ctx, principal.SchoolID, principal.UserID)
	if err != nil {
		return TeacherStats{}, err
	}
	return TeacherStats{Questions: questions, Papers: papers}, nil
}

func (r *Repository) teacherByID(ctx context.Context, schoolID, teacherID uuid.UUID) (*ent.User, error) {
	entity, err := r.client.User.Query().Where(
		entuser.SchoolIDEQ(schoolID),
		entuser.IDEQ(teacherID),
		entuser.RoleEQ(auth.RoleTeacher),
		entuser.DeletedAtIsNil(),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	return entity, err
}

func (r *Repository) countTeachers(ctx context.Context, schoolID uuid.UUID, status string) (int, error) {
	return r.client.User.Query().Where(
		entuser.SchoolIDEQ(schoolID),
		entuser.RoleEQ(auth.RoleTeacher),
		entuser.StatusEQ(status),
		entuser.DeletedAtIsNil(),
	).Count(ctx)
}

func (r *Repository) countQuestions(ctx context.Context, schoolID, creatorID uuid.UUID) (QuestionStats, error) {
	count := func(status string) (int, error) {
		predicates := []predicate.Question{
			entquestion.SchoolIDEQ(schoolID),
			entquestion.StatusEQ(status),
			entquestion.DeletedAtIsNil(),
		}
		if creatorID != uuid.Nil {
			predicates = append(predicates, entquestion.CreatorIDEQ(creatorID))
		}
		return r.client.Question.Query().Where(predicates...).Count(ctx)
	}
	draft, err := count(question.StatusDraft)
	if err != nil {
		return QuestionStats{}, err
	}
	published, err := count(question.StatusPublished)
	if err != nil {
		return QuestionStats{}, err
	}
	archived, err := count(question.StatusArchived)
	if err != nil {
		return QuestionStats{}, err
	}
	return QuestionStats{Draft: draft, Published: published, Archived: archived}, nil
}

func (r *Repository) countPapers(ctx context.Context, schoolID, creatorID uuid.UUID) (int, error) {
	predicates := []predicate.Paper{
		entpaper.SchoolIDEQ(schoolID),
		entpaper.DeletedAtIsNil(),
	}
	if creatorID != uuid.Nil {
		predicates = append(predicates, entpaper.CreatorIDEQ(creatorID))
	}
	return r.client.Paper.Query().Where(predicates...).Count(ctx)
}

func validAdmin(principal auth.Principal) bool {
	return principal.Role == auth.RoleAdmin && principal.UserID != uuid.Nil && principal.SchoolID != uuid.Nil
}

func userFromEnt(entity *ent.User) auth.User {
	return auth.User{
		ID:        entity.ID,
		SchoolID:  entity.SchoolID,
		Role:      entity.Role,
		Name:      entity.Name,
		Subject:   entity.Subject,
		Email:     entity.Email,
		Status:    entity.Status,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
}

func schoolFromEnt(entity *ent.School) School {
	return School{
		ID:         entity.ID,
		Name:       entity.Name,
		SchoolCode: entity.SchoolCode,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
}
