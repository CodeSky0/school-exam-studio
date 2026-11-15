package ai

import (
	"context"
	"encoding/json"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/aiauditlog"
	"school-exam-studio/backend/ent/airun"
	"school-exam-studio/backend/internal/auth"
)

type Repository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) CreateRun(ctx context.Context, principal auth.Principal, run AgentRun) (AgentRun, error) {
	if err := validatePrincipal(principal); err != nil {
		return AgentRun{}, err
	}
	if r.client == nil {
		return AgentRun{}, ErrNotFound
	}
	result := run.Result
	if result == nil {
		result = map[string]any{}
	}
	create := r.client.AIRun.Create().
		SetSchoolID(principal.SchoolID).
		SetCreatorID(principal.UserID).
		SetGoal(run.Goal).
		SetIntent(run.Intent).
		SetStatus(run.Status).
		SetProvider(run.Provider).
		SetModel(run.Model).
		SetPlan(stepsToMaps(run.Plan)).
		SetSteps(stepsToMaps(run.Steps)).
		SetResult(result).
		SetError(run.Error)
	if run.ID != "" {
		id, err := uuid.Parse(run.ID)
		if err != nil {
			return AgentRun{}, validationError("id", "运行ID格式不正确")
		}
		create = create.SetID(id)
	}
	entity, err := create.Save(ctx)
	if err != nil {
		return AgentRun{}, err
	}
	return runFromEnt(entity), nil
}

func (r *Repository) GetRun(ctx context.Context, principal auth.Principal, id uuid.UUID) (AgentRun, error) {
	if err := validatePrincipal(principal); err != nil {
		return AgentRun{}, err
	}
	entity, err := r.client.AIRun.Query().Where(
		airun.SchoolIDEQ(principal.SchoolID),
		airun.IDEQ(id),
	).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return AgentRun{}, ErrNotFound
		}
		return AgentRun{}, err
	}
	if principal.Role == auth.RoleTeacher && entity.CreatorID != principal.UserID {
		return AgentRun{}, ErrForbidden
	}
	return runFromEnt(entity), nil
}

func (r *Repository) ListRuns(ctx context.Context, principal auth.Principal, limit int) ([]AgentRun, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := r.client.AIRun.Query().Where(airun.SchoolIDEQ(principal.SchoolID))
	if principal.Role == auth.RoleTeacher {
		query = query.Where(airun.CreatorIDEQ(principal.UserID))
	}
	entities, err := query.
		Order(
			airun.ByCreatedAt(entsql.OrderDesc()),
			airun.ByID(entsql.OrderDesc()),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]AgentRun, 0, len(entities))
	for _, entity := range entities {
		items = append(items, runFromEnt(entity))
	}
	return items, nil
}

func (r *Repository) CreateAudit(ctx context.Context, principal auth.Principal, log AuditLog) error {
	if err := validatePrincipal(principal); err != nil {
		return err
	}
	detail := log.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	create := r.client.AIAuditLog.Create().
		SetSchoolID(principal.SchoolID).
		SetActorID(principal.UserID).
		SetAction(log.Action).
		SetTargetType(log.TargetType).
		SetDetail(detail)
	if log.RunID != "" {
		runID, err := uuid.Parse(log.RunID)
		if err == nil {
			create = create.SetRunID(runID)
		}
	}
	if log.TargetID != "" {
		targetID, err := uuid.Parse(log.TargetID)
		if err == nil {
			create = create.SetTargetID(targetID)
		}
	}
	_, err := create.Save(ctx)
	return err
}

func (r *Repository) ListAudits(ctx context.Context, principal auth.Principal, limit int) ([]AuditLog, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if principal.Role != auth.RoleAdmin {
		return nil, ErrForbidden
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	entities, err := r.client.AIAuditLog.Query().
		Where(aiauditlog.SchoolIDEQ(principal.SchoolID)).
		Order(
			aiauditlog.ByCreatedAt(entsql.OrderDesc()),
			aiauditlog.ByID(entsql.OrderDesc()),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]AuditLog, 0, len(entities))
	for _, entity := range entities {
		items = append(items, auditFromEnt(entity))
	}
	return items, nil
}

func runFromEnt(entity *ent.AIRun) AgentRun {
	return AgentRun{
		ID:        entity.ID.String(),
		Goal:      entity.Goal,
		Intent:    entity.Intent,
		Status:    entity.Status,
		Provider:  entity.Provider,
		Model:     entity.Model,
		Plan:      mapsToSteps(entity.Plan),
		Steps:     mapsToSteps(entity.Steps),
		Result:    cloneMap(entity.Result),
		Error:     entity.Error,
		CreatedAt: entity.CreatedAt,
	}
}

func auditFromEnt(entity *ent.AIAuditLog) AuditLog {
	log := AuditLog{
		ID:         entity.ID.String(),
		ActorID:    entity.ActorID.String(),
		Action:     entity.Action,
		TargetType: entity.TargetType,
		Detail:     cloneMap(entity.Detail),
		CreatedAt:  entity.CreatedAt,
	}
	if entity.RunID != nil {
		log.RunID = entity.RunID.String()
	}
	if entity.TargetID != nil {
		log.TargetID = entity.TargetID.String()
	}
	return log
}

func stepsToMaps(steps []Step) []map[string]any {
	if len(steps) == 0 {
		return []map[string]any{}
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(steps))
	if err := json.Unmarshal(raw, &result); err != nil {
		return []map[string]any{}
	}
	return result
}

func mapsToSteps(values []map[string]any) []Step {
	if len(values) == 0 {
		return []Step{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return []Step{}
	}
	result := make([]Step, 0, len(values))
	if err := json.Unmarshal(raw, &result); err != nil {
		return []Step{}
	}
	return result
}

func cloneMap(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
