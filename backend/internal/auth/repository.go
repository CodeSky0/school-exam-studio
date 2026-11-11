package auth

import (
	"context"
	"errors"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/school"
	"school-exam-studio/backend/ent/user"
)

const initializationAdvisoryLock int64 = 0x5353482d494e4954

var errSchoolCodeConflict = errors.New("school code conflict")

type Repository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) Initialized(ctx context.Context) (bool, error) {
	return r.client.School.Query().Exist(ctx)
}

func (r *Repository) Initialize(
	ctx context.Context,
	input InitializeInput,
	passwordHash string,
	schoolCode string,
) (InitializeResult, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return InitializeResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	if err := acquireInitializationLock(ctx, client); err != nil {
		return InitializeResult{}, err
	}
	initialized, err := client.School.Query().Exist(ctx)
	if err != nil {
		return InitializeResult{}, err
	}
	if initialized {
		return InitializeResult{}, ErrAlreadyInitialized
	}
	createdSchool, err := client.School.Create().
		SetName(input.SchoolName).
		SetSchoolCode(schoolCode).
		Save(ctx)
	if err != nil {
		if isConstraint(err, "schools_school_code_key") {
			return InitializeResult{}, errSchoolCodeConflict
		}
		return InitializeResult{}, err
	}
	admin, err := client.User.Create().
		SetSchoolID(createdSchool.ID).
		SetRole(RoleAdmin).
		SetName(input.AdminName).
		SetSubject("").
		SetEmail(input.Email).
		SetPasswordHash(passwordHash).
		SetStatus(StatusActive).
		SetSessionVersion(1).
		Save(ctx)
	if err != nil {
		return InitializeResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return InitializeResult{}, err
	}
	return InitializeResult{
		SchoolID:   createdSchool.ID,
		SchoolCode: createdSchool.SchoolCode,
		User:       userFromEnt(admin),
	}, nil
}

func acquireInitializationLock(ctx context.Context, client *ent.Client) error {
	var result []struct {
		Locked string `json:"locked"`
	}
	err := client.School.Query().Aggregate(ent.As(func(selector *entsql.Selector) string {
		selector.FromExpr(entsql.Expr("(SELECT 1) AS schools"))
		selector.SetP(nil)
		return fmt.Sprintf("pg_advisory_xact_lock(%d)", initializationAdvisoryLock)
	}, "locked")).Scan(ctx, &result)
	if err != nil {
		return fmt.Errorf("acquire initialization advisory lock: %w", err)
	}
	if len(result) != 1 {
		return errors.New("initialization advisory lock returned no result")
	}
	return nil
}

func (r *Repository) SchoolByCode(ctx context.Context, schoolCode string) (*ent.School, error) {
	return r.client.School.Query().Where(school.SchoolCodeEQ(schoolCode)).Only(ctx)
}

func (r *Repository) SingleSchool(ctx context.Context) (*ent.School, error) {
	return r.client.School.Query().Only(ctx)
}

func (r *Repository) CreateTeacher(ctx context.Context, input RegisterInput, passwordHash string) (User, error) {
	schoolEntity, err := r.SchoolByCode(ctx, input.SchoolCode)
	if ent.IsNotFound(err) {
		return User{}, ErrInvalidSchoolCode
	}
	if err != nil {
		return User{}, err
	}
	teacher, err := r.client.User.Create().
		SetSchoolID(schoolEntity.ID).
		SetRole(RoleTeacher).
		SetName(input.Name).
		SetSubject(input.Subject).
		SetEmail(input.Email).
		SetPasswordHash(passwordHash).
		SetStatus(StatusActive).
		SetSessionVersion(1).
		Save(ctx)
	if isConstraint(err, "users_school_email_key") {
		return User{}, ErrEmailAlreadyExists
	}
	if err != nil {
		return User{}, err
	}
	return userFromEnt(teacher), nil
}

func (r *Repository) UserByEmail(ctx context.Context, schoolID uuid.UUID, email string) (*ent.User, error) {
	return r.client.User.Query().Where(
		user.SchoolIDEQ(schoolID),
		user.EmailEQ(email),
	).Only(ctx)
}

func (r *Repository) UserByID(ctx context.Context, schoolID, userID uuid.UUID) (*ent.User, error) {
	return r.client.User.Query().Where(
		user.SchoolIDEQ(schoolID),
		user.IDEQ(userID),
	).Only(ctx)
}

func (r *Repository) UpdatePassword(ctx context.Context, principal Principal, passwordHash string) error {
	updated, err := r.client.User.Update().Where(
		user.SchoolIDEQ(principal.SchoolID),
		user.IDEQ(principal.UserID),
		user.SessionVersionEQ(principal.SessionVersion),
		user.StatusEQ(StatusActive),
	).
		SetPasswordHash(passwordHash).
		AddSessionVersion(1).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated != 1 {
		return ErrSessionInvalid
	}
	return nil
}

func userFromEnt(entity *ent.User) User {
	return User{
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

func isConstraint(err error, name string) bool {
	var postgresError *pq.Error
	return errors.As(err, &postgresError) && postgresError.Constraint == name
}
