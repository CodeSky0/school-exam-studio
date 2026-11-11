package importer

import (
	"context"
	"errors"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/predicate"
	"school-exam-studio/backend/ent/schema"
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

func (r *Repository) Creator(ctx context.Context, principal auth.Principal) (*ent.User, error) {
	if !validImportPrincipal(principal) {
		return nil, ErrForbidden
	}
	creator, err := r.client.User.Query().Where(
		entuser.SchoolIDEQ(principal.SchoolID),
		entuser.IDEQ(principal.UserID),
		entuser.RoleEQ(principal.Role),
		entuser.StatusEQ(auth.StatusActive),
		entuser.DeletedAtIsNil(),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrForbidden
	}
	return creator, err
}

func (r *Repository) Import(ctx context.Context, principal auth.Principal, rows []ImportRow) (int, error) {
	if !validImportPrincipal(principal) {
		return 0, ErrForbidden
	}
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	creator, err := tx.User.Query().Where(
		entuser.SchoolIDEQ(principal.SchoolID),
		entuser.IDEQ(principal.UserID),
		entuser.RoleEQ(principal.Role),
		entuser.StatusEQ(auth.StatusActive),
		entuser.DeletedAtIsNil(),
		predicate.User(func(selector *entsql.Selector) { selector.ForUpdate() }),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return 0, ErrForbidden
	}
	if err != nil {
		return 0, err
	}
	builders := make([]*ent.QuestionCreate, 0, len(rows))
	for _, row := range rows {
		if principal.Role == auth.RoleTeacher && creator.Subject != row.Subject {
			return 0, ErrForbidden
		}
		builders = append(builders, tx.Question.Create().
			SetSchoolID(principal.SchoolID).
			SetCreatorID(principal.UserID).
			SetSubject(row.Subject).
			SetGrade(row.Grade).
			SetQuestionType(row.Type).
			SetDifficulty(row.Difficulty).
			SetTags(row.Tags).
			SetContent(row.Content).
			SetOptions(schemaOptions(row.Options)).
			SetAnswer(row.Answer).
			SetExplanation(row.Explanation).
			SetStatus(question.StatusDraft))
	}
	created, err := tx.Question.CreateBulk(builders...).Save(ctx)
	if err != nil {
		return 0, err
	}
	if len(created) != len(rows) {
		return 0, errors.New("CSV import created an unexpected question count")
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(created), nil
}

func schemaOptions(options []question.Option) []schema.Option {
	result := make([]schema.Option, 0, len(options))
	for _, option := range options {
		result = append(result, schema.Option{Key: option.Key, Content: option.Content})
	}
	return result
}

func validImportPrincipal(principal auth.Principal) bool {
	return principal.UserID != uuid.Nil && principal.SchoolID != uuid.Nil &&
		(principal.Role == auth.RoleTeacher || principal.Role == auth.RoleAdmin)
}

