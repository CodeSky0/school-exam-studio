package ai

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/schoolaisetting"
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

func (r *Repository) CreateAudit(ctx context.Context, principal auth.Principal, log AuditLog) error {
	if err := validatePrincipal(principal); err != nil {
		return err
	}
	if r.client == nil {
		return nil
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

func (r *Repository) GetSchoolAISetting(ctx context.Context, schoolID uuid.UUID) (schoolAISettingRecord, error) {
	if r.client == nil {
		return schoolAISettingRecord{}, ErrNotFound
	}
	entity, err := r.client.SchoolAISetting.Query().Where(
		schoolaisetting.SchoolIDEQ(schoolID),
		schoolaisetting.DeletedAtIsNil(),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return schoolAISettingRecord{}, ErrNotFound
	}
	if err != nil {
		return schoolAISettingRecord{}, err
	}
	return schoolAISettingFromEnt(entity), nil
}

func (r *Repository) SaveSchoolAISetting(ctx context.Context, principal auth.Principal, record schoolAISettingRecord) (schoolAISettingRecord, error) {
	if principal.Role != auth.RoleAdmin {
		return schoolAISettingRecord{}, ErrForbidden
	}
	if r.client == nil {
		return schoolAISettingRecord{}, ErrNotFound
	}
	existing, err := r.client.SchoolAISetting.Query().Where(
		schoolaisetting.SchoolIDEQ(principal.SchoolID),
		schoolaisetting.DeletedAtIsNil(),
	).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return schoolAISettingRecord{}, err
	}
	if ent.IsNotFound(err) {
		entity, createErr := r.client.SchoolAISetting.Create().
			SetSchoolID(principal.SchoolID).
			SetProvider(record.Provider).
			SetBaseURL(record.BaseURL).
			SetAPIKey(record.APIKey).
			SetModels(record.Models).
			SetDefaultModel(record.DefaultModel).
			SetEnabled(record.Enabled).
			SetUpdatedBy(principal.UserID).
			Save(ctx)
		if createErr != nil {
			return schoolAISettingRecord{}, createErr
		}
		return schoolAISettingFromEnt(entity), nil
	}
	entity, err := existing.Update().
		SetProvider(record.Provider).
		SetBaseURL(record.BaseURL).
		SetAPIKey(record.APIKey).
		SetModels(record.Models).
		SetDefaultModel(record.DefaultModel).
		SetEnabled(record.Enabled).
		SetUpdatedBy(principal.UserID).
		Save(ctx)
	if err != nil {
		return schoolAISettingRecord{}, err
	}
	return schoolAISettingFromEnt(entity), nil
}

func schoolAISettingFromEnt(entity *ent.SchoolAISetting) schoolAISettingRecord {
	return schoolAISettingRecord{
		Provider:     entity.Provider,
		BaseURL:      entity.BaseURL,
		APIKey:       entity.APIKey,
		Models:       entity.Models,
		DefaultModel: entity.DefaultModel,
		Enabled:      entity.Enabled,
		UpdatedAt:    entity.UpdatedAt,
	}
}
