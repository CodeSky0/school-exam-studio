package paper

import (
	"context"
	"errors"
	"sort"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	entpaper "school-exam-studio/backend/ent/paper"
	entpaperquestion "school-exam-studio/backend/ent/paperquestion"
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

func (r *Repository) create(ctx context.Context, principal auth.Principal, input normalizedInput) (Paper, error) {
	if !validPrincipal(principal) {
		return Paper{}, ErrForbidden
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return Paper{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockQuestions(ctx, tx, principal.SchoolID, input); err != nil {
		return Paper{}, err
	}
	created, err := tx.Paper.Create().
		SetSchoolID(principal.SchoolID).
		SetCreatorID(principal.UserID).
		SetTitle(input.title).
		SetSubject(input.subject).
		SetGrade(input.grade).
		SetTotalScore("0.00").
		SetDurationMinutes(input.durationMinutes).
		Save(ctx)
	if err != nil {
		return Paper{}, err
	}
	if err := replaceQuestionsAndSaveTotal(ctx, tx, principal.SchoolID, created.ID, input); err != nil {
		return Paper{}, err
	}
	if err := tx.Commit(); err != nil {
		return Paper{}, err
	}
	return r.Get(ctx, principal, created.ID)
}

func (r *Repository) update(ctx context.Context, principal auth.Principal, id uuid.UUID, input normalizedInput) (Paper, error) {
	if !validPrincipal(principal) {
		return Paper{}, ErrForbidden
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return Paper{}, err
	}
	defer func() { _ = tx.Rollback() }()
	existing, err := lockPaper(ctx, tx, principal.SchoolID, id)
	if err != nil {
		return Paper{}, err
	}
	if err := checkOwnership(principal, existing); err != nil {
		return Paper{}, err
	}
	if _, err := lockQuestions(ctx, tx, principal.SchoolID, input); err != nil {
		return Paper{}, err
	}
	updated, err := tx.Paper.Update().Where(
		entpaper.SchoolIDEQ(principal.SchoolID),
		entpaper.IDEQ(id),
	).
		SetTitle(input.title).
		SetSubject(input.subject).
		SetGrade(input.grade).
		SetDurationMinutes(input.durationMinutes).
		Save(ctx)
	if err != nil {
		return Paper{}, err
	}
	if updated != 1 {
		return Paper{}, ErrNotFound
	}
	if err := replaceQuestionsAndSaveTotal(ctx, tx, principal.SchoolID, id, input); err != nil {
		return Paper{}, err
	}
	if err := tx.Commit(); err != nil {
		return Paper{}, err
	}
	return r.Get(ctx, principal, id)
}

func (r *Repository) Get(ctx context.Context, principal auth.Principal, id uuid.UUID) (Paper, error) {
	if !validPrincipal(principal) {
		return Paper{}, ErrForbidden
	}
	entity, err := r.client.Paper.Query().Where(
		entpaper.SchoolIDEQ(principal.SchoolID),
		entpaper.IDEQ(id),
	).
		WithSchool(func(query *ent.SchoolQuery) {
			query.Where(entschool.IDEQ(principal.SchoolID))
		}).
		WithPaperQuestions(func(query *ent.PaperQuestionQuery) {
			query.Where(entpaperquestion.SchoolIDEQ(principal.SchoolID)).
				Order(entpaperquestion.ByOrderIndex()).
				WithQuestion(func(questionQuery *ent.QuestionQuery) {
					questionQuery.Where(entquestion.SchoolIDEQ(principal.SchoolID)).
						WithCreator(func(userQuery *ent.UserQuery) {
							userQuery.Where(entuser.SchoolIDEQ(principal.SchoolID))
						})
				})
		}).
		Only(ctx)
	if ent.IsNotFound(err) {
		return Paper{}, ErrNotFound
	}
	if err != nil {
		return Paper{}, err
	}
	if err := checkOwnership(principal, entity); err != nil {
		return Paper{}, err
	}
	return fromEnt(entity, true)
}

func (r *Repository) List(ctx context.Context, principal auth.Principal, filter Filter) (ListResult, error) {
	if !validPrincipal(principal) {
		return ListResult{}, ErrForbidden
	}
	predicates := []predicate.Paper{entpaper.SchoolIDEQ(principal.SchoolID)}
	if principal.Role == auth.RoleTeacher {
		predicates = append(predicates, entpaper.CreatorIDEQ(principal.UserID))
	}
	if filter.Subject != "" {
		predicates = append(predicates, entpaper.SubjectEQ(filter.Subject))
	}
	if filter.Grade != "" {
		predicates = append(predicates, entpaper.GradeEQ(filter.Grade))
	}
	query := r.client.Paper.Query().Where(predicates...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return ListResult{}, err
	}
	entities, err := query.
		WithSchool(func(schoolQuery *ent.SchoolQuery) {
			schoolQuery.Where(entschool.IDEQ(principal.SchoolID))
		}).
		Order(
			entpaper.ByUpdatedAt(entsql.OrderDesc()),
			entpaper.ByID(entsql.OrderDesc()),
		).
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		All(ctx)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]Paper, 0, len(entities))
	for _, entity := range entities {
		item, err := fromEnt(entity, false)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	return ListResult{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (r *Repository) Delete(ctx context.Context, principal auth.Principal, id uuid.UUID) error {
	if !validPrincipal(principal) {
		return ErrForbidden
	}
	entity, err := r.client.Paper.Query().Where(
		entpaper.SchoolIDEQ(principal.SchoolID),
		entpaper.IDEQ(id),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := checkOwnership(principal, entity); err != nil {
		return err
	}
	update := r.client.Paper.Update().Where(
		entpaper.SchoolIDEQ(principal.SchoolID),
		entpaper.IDEQ(id),
		entpaper.DeletedAtIsNil(),
	)
	if principal.Role == auth.RoleTeacher {
		update = update.Where(entpaper.CreatorIDEQ(principal.UserID))
	}
	deleted, err := update.SetDeletedAt(time.Now().UTC()).Save(ctx)
	if err != nil {
		return err
	}
	if deleted != 1 {
		return ErrNotFound
	}
	return nil
}

func lockPaper(ctx context.Context, tx *ent.Tx, schoolID, id uuid.UUID) (*ent.Paper, error) {
	entity, err := tx.Paper.Query().Where(
		entpaper.SchoolIDEQ(schoolID),
		entpaper.IDEQ(id),
		predicate.Paper(func(selector *entsql.Selector) {
			selector.ForUpdate()
		}),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func lockQuestions(ctx context.Context, tx *ent.Tx, schoolID uuid.UUID, input normalizedInput) (map[uuid.UUID]*ent.Question, error) {
	ids := make([]uuid.UUID, 0, len(input.questions))
	for _, item := range input.questions {
		ids = append(ids, item.questionID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	entities, err := tx.Question.Query().Where(
		entquestion.SchoolIDEQ(schoolID),
		entquestion.StatusEQ(question.StatusPublished),
		entquestion.IDIn(ids...),
		predicate.Question(func(selector *entsql.Selector) {
			selector.ForUpdate()
		}),
	).Order(entquestion.ByID()).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(entities) != len(ids) {
		return nil, ErrQuestionUnavailable
	}
	byID := make(map[uuid.UUID]*ent.Question, len(entities))
	for _, entity := range entities {
		byID[entity.ID] = entity
	}
	for _, item := range input.questions {
		entity := byID[item.questionID]
		if entity.Subject != input.subject || entity.Grade != input.grade {
			return nil, ErrQuestionMismatch
		}
	}
	return byID, nil
}

func replaceQuestionsAndSaveTotal(ctx context.Context, tx *ent.Tx, schoolID, paperID uuid.UUID, input normalizedInput) error {
	if _, err := tx.PaperQuestion.Update().Where(
		entpaperquestion.SchoolIDEQ(schoolID),
		entpaperquestion.PaperIDEQ(paperID),
		entpaperquestion.DeletedAtIsNil(),
	).SetDeletedAt(time.Now().UTC()).Save(ctx); err != nil {
		return err
	}
	scores := make([]Score, 0, len(input.questions))
	for index, item := range input.questions {
		if _, err := tx.PaperQuestion.Create().
			SetSchoolID(schoolID).
			SetPaperID(paperID).
			SetQuestionID(item.questionID).
			SetOrderIndex(index).
			SetAssignedScore(item.score.String()).
			Save(ctx); err != nil {
			return err
		}
		scores = append(scores, item.score)
	}
	total := SumScores(scores)
	if total.exceedsMaximum() {
		return validationError("questions", "试卷总分不能超过999999.99")
	}
	updated, err := tx.Paper.Update().Where(
		entpaper.SchoolIDEQ(schoolID),
		entpaper.IDEQ(paperID),
	).SetTotalScore(total.String()).Save(ctx)
	if err != nil {
		return err
	}
	if updated != 1 {
		return ErrNotFound
	}
	return nil
}

func checkOwnership(principal auth.Principal, entity *ent.Paper) error {
	if principal.Role == auth.RoleTeacher && entity.CreatorID != principal.UserID {
		return ErrForbidden
	}
	return nil
}

func fromEnt(entity *ent.Paper, includeQuestions bool) (Paper, error) {
	total, err := ParseScore(entity.TotalScore)
	if err != nil {
		return Paper{}, err
	}
	result := Paper{
		ID:              entity.ID,
		CreatorID:       entity.CreatorID,
		Title:           entity.Title,
		Subject:         entity.Subject,
		Grade:           entity.Grade,
		TotalScore:      total.String(),
		DurationMinutes: entity.DurationMinutes,
		CreatedAt:       entity.CreatedAt,
		UpdatedAt:       entity.UpdatedAt,
	}
	if entity.Edges.School != nil {
		result.SchoolName = entity.Edges.School.Name
	}
	if !includeQuestions {
		return result, nil
	}
	result.Questions = make([]PaperQuestion, 0, len(entity.Edges.PaperQuestions))
	for _, detail := range entity.Edges.PaperQuestions {
		if detail.Edges.Question == nil {
			return Paper{}, errors.New("paper question relation is missing")
		}
		assigned, err := ParseScore(detail.AssignedScore)
		if err != nil {
			return Paper{}, err
		}
		result.Questions = append(result.Questions, PaperQuestion{
			QuestionID:    detail.QuestionID,
			OrderIndex:    detail.OrderIndex,
			AssignedScore: assigned.String(),
			Question:      questionFromEnt(detail.Edges.Question),
		})
	}
	return result, nil
}

func questionFromEnt(entity *ent.Question) question.Question {
	options := make([]question.Option, 0, len(entity.Options))
	for _, option := range entity.Options {
		options = append(options, question.Option{Key: option.Key, Content: option.Content})
	}
	creatorName := ""
	if entity.Edges.Creator != nil {
		creatorName = entity.Edges.Creator.Name
	}
	return question.Question{
		ID:           entity.ID,
		CreatorID:    entity.CreatorID,
		CreatorName:  creatorName,
		Subject:      entity.Subject,
		Grade:        entity.Grade,
		QuestionType: entity.QuestionType,
		Difficulty:   entity.Difficulty,
		Tags:         append([]string{}, entity.Tags...),
		Content:      entity.Content,
		Options:      options,
		Answer:       entity.Answer,
		Explanation:  entity.Explanation,
		Status:       entity.Status,
		CreatedAt:    entity.CreatedAt,
		UpdatedAt:    entity.UpdatedAt,
	}
}

