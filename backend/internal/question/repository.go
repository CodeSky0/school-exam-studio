package question

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/predicate"
	entquestion "school-exam-studio/backend/ent/question"
	"school-exam-studio/backend/ent/schema"
	"school-exam-studio/backend/ent/user"
	"school-exam-studio/backend/internal/auth"
)

type Repository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) Create(ctx context.Context, principal auth.Principal, input Input) (Question, error) {
	if err := r.checkCreator(ctx, principal, input.Subject); err != nil {
		return Question{}, err
	}
	create := r.client.Question.Create().
		SetSchoolID(principal.SchoolID).
		SetCreatorID(principal.UserID).
		SetSubject(input.Subject).
		SetGrade(input.Grade).
		SetQuestionType(input.QuestionType).
		SetDifficulty(input.Difficulty).
		SetTags(input.Tags).
		SetCategory(input.Category).
		SetKnowledgePoints(input.KnowledgePoints).
		SetContent(input.Content).
		SetOptions(toSchemaOptions(input.Options)).
		SetAnswer(input.Answer).
		SetExplanation(input.Explanation).
		SetStatus(StatusDraft).
		SetAiGenerated(input.AIGenerated).
		SetContentHash(ContentHash(input))
	if input.AIMeta != nil {
		create = create.SetAiMeta(input.AIMeta)
	}
	if input.SourceQuestionID != uuid.Nil {
		create = create.SetSourceQuestionID(input.SourceQuestionID)
	}
	created, err := create.Save(ctx)
	if err != nil {
		return Question{}, err
	}
	return r.Get(ctx, principal, created.ID)
}

func (r *Repository) Get(ctx context.Context, principal auth.Principal, id uuid.UUID) (Question, error) {
	if !validPrincipal(principal) {
		return Question{}, ErrForbidden
	}
	predicates := []predicate.Question{
		entquestion.SchoolIDEQ(principal.SchoolID),
		entquestion.IDEQ(id),
	}
	if principal.Role == auth.RoleTeacher {
		predicates = append(predicates, teacherVisible(principal.UserID))
	}
	entity, err := r.client.Question.Query().Where(predicates...).WithCreator(func(query *ent.UserQuery) {
		query.Where(user.SchoolIDEQ(principal.SchoolID))
	}).Only(ctx)
	if ent.IsNotFound(err) {
		return Question{}, ErrNotFound
	}
	if err != nil {
		return Question{}, err
	}
	return fromEnt(entity), nil
}

func (r *Repository) List(ctx context.Context, principal auth.Principal, filter QuestionFilter) (ListResult, error) {
	if !validPrincipal(principal) {
		return ListResult{}, ErrForbidden
	}
	predicates := []predicate.Question{entquestion.SchoolIDEQ(principal.SchoolID)}
	if principal.Role == auth.RoleTeacher {
		predicates = append(predicates, teacherVisible(principal.UserID))
	}
	if filter.Subject != "" {
		predicates = append(predicates, entquestion.SubjectEQ(filter.Subject))
	}
	if filter.Grade != "" {
		predicates = append(predicates, entquestion.GradeEQ(filter.Grade))
	}
	if filter.Type != "" {
		predicates = append(predicates, entquestion.QuestionTypeEQ(filter.Type))
	}
	if filter.Difficulty != 0 {
		predicates = append(predicates, entquestion.DifficultyEQ(filter.Difficulty))
	}
	if filter.Status != "" {
		predicates = append(predicates, entquestion.StatusEQ(filter.Status))
	}
	if filter.CreatorID != "" {
		creatorID, err := uuid.Parse(filter.CreatorID)
		if err != nil {
			return ListResult{}, validationError("creator_id", "创建人ID格式不正确")
		}
		predicates = append(predicates, entquestion.CreatorIDEQ(creatorID))
	}
	if filter.Category != "" {
		predicates = append(predicates, entquestion.CategoryEQ(filter.Category))
	}
	if filter.Tag != "" {
		predicates = append(predicates, jsonArrayContains(entquestion.FieldTags, filter.Tag))
	}
	if filter.KnowledgePoint != "" {
		predicates = append(predicates, jsonArrayContains(entquestion.FieldKnowledgePoints, filter.KnowledgePoint))
	}
	query := r.client.Question.Query().Where(predicates...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return ListResult{}, err
	}
	entities, err := query.
		WithCreator(func(query *ent.UserQuery) {
			query.Where(user.SchoolIDEQ(principal.SchoolID))
		}).
		Order(
			entquestion.ByCreatedAt(entsql.OrderDesc()),
			entquestion.ByID(entsql.OrderDesc()),
		).
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		All(ctx)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]Question, 0, len(entities))
	for _, entity := range entities {
		items = append(items, fromEnt(entity))
	}
	return ListResult{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (r *Repository) Update(ctx context.Context, principal auth.Principal, id uuid.UUID, input Input) (Question, error) {
	if !validPrincipal(principal) {
		return Question{}, ErrForbidden
	}
	if err := r.checkCreator(ctx, principal, input.Subject); err != nil {
		return Question{}, err
	}
	entity, err := r.schoolQuestion(ctx, principal.SchoolID, id)
	if err != nil {
		return Question{}, err
	}
	if entity.Status != StatusDraft || (principal.Role == auth.RoleTeacher && entity.CreatorID != principal.UserID) {
		return Question{}, ErrForbidden
	}
	update := r.client.Question.Update().Where(
		entquestion.SchoolIDEQ(principal.SchoolID),
		entquestion.IDEQ(id),
		entquestion.StatusEQ(StatusDraft),
	)
	if principal.Role == auth.RoleTeacher {
		update = update.Where(
			entquestion.CreatorIDEQ(principal.UserID),
			entquestion.StatusEQ(StatusDraft),
		)
	}
	updated, err := update.
		SetSubject(input.Subject).
		SetGrade(input.Grade).
		SetQuestionType(input.QuestionType).
		SetDifficulty(input.Difficulty).
		SetTags(input.Tags).
		SetCategory(input.Category).
		SetKnowledgePoints(input.KnowledgePoints).
		SetContent(input.Content).
		SetOptions(toSchemaOptions(input.Options)).
		SetAnswer(input.Answer).
		SetExplanation(input.Explanation).
		SetContentHash(ContentHash(input)).
		Save(ctx)
	if err != nil {
		return Question{}, err
	}
	if updated != 1 {
		return Question{}, ErrForbidden
	}
	return r.Get(ctx, principal, id)
}

func (r *Repository) Delete(ctx context.Context, principal auth.Principal, id uuid.UUID) error {
	if !validPrincipal(principal) {
		return ErrForbidden
	}
	entity, err := r.schoolQuestion(ctx, principal.SchoolID, id)
	if err != nil {
		return err
	}
	if entity.Status != StatusDraft || (principal.Role == auth.RoleTeacher && entity.CreatorID != principal.UserID) {
		return ErrForbidden
	}
	update := r.client.Question.Update().Where(
		entquestion.SchoolIDEQ(principal.SchoolID),
		entquestion.IDEQ(id),
		entquestion.StatusEQ(StatusDraft),
	)
	if principal.Role == auth.RoleTeacher {
		update = update.Where(
			entquestion.CreatorIDEQ(principal.UserID),
			entquestion.StatusEQ(StatusDraft),
		)
	}
	updated, err := update.SetDeletedAt(time.Now().UTC()).Save(ctx)
	if err != nil {
		return err
	}
	if updated != 1 {
		return ErrForbidden
	}
	return nil
}

func (r *Repository) ResolvePublished(ctx context.Context, principal auth.Principal, ids []uuid.UUID) ([]Question, error) {
	if !validPrincipal(principal) {
		return nil, ErrForbidden
	}
	uniqueIDs := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return []Question{}, nil
	}
	entities, err := r.client.Question.Query().Where(
		entquestion.SchoolIDEQ(principal.SchoolID),
		entquestion.StatusEQ(StatusPublished),
		entquestion.IDIn(uniqueIDs...),
	).WithCreator(func(query *ent.UserQuery) {
		query.Where(user.SchoolIDEQ(principal.SchoolID))
	}).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]Question, len(entities))
	for _, entity := range entities {
		byID[entity.ID] = fromEnt(entity)
	}
	items := make([]Question, 0, len(entities))
	for _, id := range uniqueIDs {
		if item, exists := byID[id]; exists {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, principal auth.Principal, id uuid.UUID, status string) (Question, error) {
	if !validPrincipal(principal) || principal.Role != auth.RoleAdmin {
		return Question{}, ErrForbidden
	}
	entity, err := r.schoolQuestion(ctx, principal.SchoolID, id)
	if err != nil {
		return Question{}, err
	}
	validTransition := (entity.Status == StatusDraft && status == StatusPublished) ||
		((entity.Status == StatusDraft || entity.Status == StatusPublished) && status == StatusArchived)
	if !validTransition {
		return Question{}, ErrInvalidTransition
	}
	updated, err := r.client.Question.Update().Where(
		entquestion.SchoolIDEQ(principal.SchoolID),
		entquestion.IDEQ(id),
		entquestion.StatusEQ(entity.Status),
	).SetStatus(status).Save(ctx)
	if err != nil {
		return Question{}, err
	}
	if updated != 1 {
		return Question{}, ErrInvalidTransition
	}
	return r.Get(ctx, principal, id)
}

func (r *Repository) Snapshot(ctx context.Context, principal auth.Principal, limit int) ([]Question, error) {
	if !validPrincipal(principal) {
		return nil, ErrForbidden
	}
	if limit < 1 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	predicates := []predicate.Question{entquestion.SchoolIDEQ(principal.SchoolID)}
	if principal.Role == auth.RoleTeacher {
		predicates = append(predicates, teacherVisible(principal.UserID))
	}
	entities, err := r.client.Question.Query().Where(predicates...).
		WithCreator(func(query *ent.UserQuery) {
			query.Where(user.SchoolIDEQ(principal.SchoolID))
		}).
		Order(
			entquestion.ByCreatedAt(entsql.OrderDesc()),
			entquestion.ByID(entsql.OrderDesc()),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Question, 0, len(entities))
	for _, entity := range entities {
		items = append(items, fromEnt(entity))
	}
	return items, nil
}

// ApplyAI 只允许修改草稿；教师仅能修改自己的草稿，管理员可修改本校草稿。
func (r *Repository) ApplyAI(ctx context.Context, principal auth.Principal, id uuid.UUID, patch AIApplyPatch) (Question, error) {
	if !validPrincipal(principal) {
		return Question{}, ErrForbidden
	}
	entity, err := r.schoolQuestion(ctx, principal.SchoolID, id)
	if err != nil {
		return Question{}, err
	}
	if entity.Status != StatusDraft {
		return Question{}, ErrForbidden
	}
	if principal.Role == auth.RoleTeacher && entity.CreatorID != principal.UserID {
		return Question{}, ErrForbidden
	}
	meta := make(map[string]any, len(entity.AiMeta)+6)
	for key, value := range entity.AiMeta {
		meta[key] = value
	}
	meta["provider"] = patch.Provider
	meta["model"] = patch.Model
	meta["confidence"] = patch.Confidence
	meta["analyzed_at"] = time.Now().UTC().Format(time.RFC3339)
	meta["review_required"] = patch.ReviewRequired
	if patch.Rationale != "" {
		meta["rationale"] = patch.Rationale
	}
	update := r.client.Question.Update().Where(
		entquestion.SchoolIDEQ(principal.SchoolID),
		entquestion.IDEQ(id),
		entquestion.StatusEQ(StatusDraft),
	)
	if principal.Role == auth.RoleTeacher {
		update = update.Where(entquestion.CreatorIDEQ(principal.UserID))
	}
	if patch.Category != nil {
		update = update.SetCategory(*patch.Category)
	}
	if patch.KnowledgePoints != nil {
		update = update.SetKnowledgePoints(*patch.KnowledgePoints)
	}
	if patch.Tags != nil {
		update = update.SetTags(*patch.Tags)
	}
	if patch.Difficulty != nil {
		update = update.SetDifficulty(*patch.Difficulty)
	}
	if patch.Answer != nil {
		update = update.SetAnswer(*patch.Answer)
	}
	if patch.Explanation != nil {
		update = update.SetExplanation(*patch.Explanation)
	}
	updated, err := update.SetAiMeta(meta).Save(ctx)
	if err != nil {
		return Question{}, err
	}
	if updated != 1 {
		return Question{}, ErrForbidden
	}
	return r.Get(ctx, principal, id)
}

func jsonArrayContains(field string, value string) predicate.Question {
	return predicate.Question(func(selector *entsql.Selector) {
		selector.Where(entsql.P(func(builder *entsql.Builder) {
			builder.WriteString(selector.C(field)).WriteString(" @> ").Arg(mustJSONArray(value))
		}))
	})
}

func mustJSONArray(value string) string {
	encoded, err := json.Marshal([]string{value})
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func (r *Repository) schoolQuestion(ctx context.Context, schoolID, id uuid.UUID) (*ent.Question, error) {
	entity, err := r.client.Question.Query().Where(
		entquestion.SchoolIDEQ(schoolID),
		entquestion.IDEQ(id),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (r *Repository) checkCreator(ctx context.Context, principal auth.Principal, subject string) error {
	if !validPrincipal(principal) {
		return ErrForbidden
	}
	creator, err := r.client.User.Query().Where(
		user.SchoolIDEQ(principal.SchoolID),
		user.IDEQ(principal.UserID),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if creator.Role != principal.Role || creator.Status != auth.StatusActive {
		return ErrForbidden
	}
	if principal.Role == auth.RoleTeacher && creator.Subject != subject {
		return ErrSubjectMismatch
	}
	return nil
}

func teacherVisible(userID uuid.UUID) predicate.Question {
	return entquestion.Or(
		entquestion.CreatorIDEQ(userID),
		entquestion.And(
			entquestion.CreatorIDNEQ(userID),
			entquestion.StatusEQ(StatusPublished),
		),
	)
}

func validPrincipal(principal auth.Principal) bool {
	return principal.UserID != uuid.Nil && principal.SchoolID != uuid.Nil &&
		(principal.Role == auth.RoleTeacher || principal.Role == auth.RoleAdmin)
}

func toSchemaOptions(options []Option) []schema.Option {
	result := make([]schema.Option, 0, len(options))
	for _, option := range options {
		result = append(result, schema.Option{Key: option.Key, Content: option.Content})
	}
	return result
}

func fromEnt(entity *ent.Question) Question {
	options := make([]Option, 0, len(entity.Options))
	for _, option := range entity.Options {
		options = append(options, Option{Key: option.Key, Content: option.Content})
	}
	tags := append([]string{}, entity.Tags...)
	knowledgePoints := append([]string{}, entity.KnowledgePoints...)
	aiMeta := make(map[string]any, len(entity.AiMeta))
	for key, value := range entity.AiMeta {
		aiMeta[key] = value
	}
	creatorName := ""
	if entity.Edges.Creator != nil {
		creatorName = entity.Edges.Creator.Name
	}
	return Question{
		ID:               entity.ID,
		CreatorID:        entity.CreatorID,
		CreatorName:      creatorName,
		Subject:          entity.Subject,
		Grade:            entity.Grade,
		QuestionType:     entity.QuestionType,
		Difficulty:       entity.Difficulty,
		Tags:             tags,
		Category:         entity.Category,
		KnowledgePoints:  knowledgePoints,
		Content:          entity.Content,
		Options:          options,
		Answer:           entity.Answer,
		Explanation:      entity.Explanation,
		Status:           entity.Status,
		AIGenerated:      entity.AiGenerated,
		AIMeta:           aiMeta,
		ContentHash:      entity.ContentHash,
		SourceQuestionID: entity.SourceQuestionID,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}
