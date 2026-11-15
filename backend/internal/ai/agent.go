package ai

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/paper"
	"school-exam-studio/backend/internal/question"
)

const (
	IntentAuditBank         = "audit_bank"
	IntentTagQuestions      = "tag_questions"
	IntentFillExplanations  = "fill_explanations"
	IntentOrganizeBank      = "organize_bank"
	IntentGenerateVariants  = "generate_variants"
	IntentGenerateQuestions = "generate_questions"
	IntentComposePaper      = "compose_paper"
	IntentAnalyzeQuestion   = "analyze_question"
	IntentUnknown           = "unknown"

	stepSucceeded = "succeeded"
	stepFailed    = "failed"
	stepSkipped   = "skipped"
	stepPlanned   = "planned"
)

type AgentPlan struct {
	Intent  string
	Title   string
	Params  AgentParams
	Steps   []Step
	Message string
}

var (
	agentCountPattern        = regexp.MustCompile(`(\d{1,2})\s*(?:道|个|条|题)`)
	agentChineseCountPattern = regexp.MustCompile(`([一二三四五六七八九十])\s*(?:道|个|条|题)`)
	agentQuotedPattern       = regexp.MustCompile(`[「《“"]([^」》”"]{2,30})[」》”"]`)
)

var chineseNumber = map[string]int{
	"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
	"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
}

// PlanGoal 是确定性意图规划器，不依赖模型，便于测试与离线运行。
func PlanGoal(request AgentRequest) (AgentPlan, error) {
	request.Goal = strings.TrimSpace(request.Goal)
	if request.Goal == "" {
		return AgentPlan{}, validationError("goal", "请描述希望 AI 完成的目标")
	}
	if len([]rune(request.Goal)) > 500 {
		return AgentPlan{}, validationError("goal", "目标描述不能超过500字")
	}
	params := AgentParams{
		Subject:        firstNonEmpty(request.Subject, detectSubject(request.Goal)),
		Grade:          firstNonEmpty(request.Grade, detectGrade(request.Goal)),
		Category:       strings.TrimSpace(request.Category),
		KnowledgePoint: strings.TrimSpace(request.KnowledgePoint),
		Tag:            strings.TrimSpace(request.Tag),
		QuestionID:     strings.TrimSpace(request.QuestionID),
		Count:          request.Count,
	}
	if params.KnowledgePoint == "" {
		params.KnowledgePoint = extractQuoted(request.Goal)
	}
	if params.KnowledgePoint == "" {
		params.KnowledgePoint = matchKnowledgePoint(request.Goal)
	}
	if params.Count <= 0 {
		params.Count = extractCount(request.Goal)
	}
	intent, title, message := detectIntent(request)
	if params.Count <= 0 {
		switch intent {
		case IntentComposePaper:
			params.Count = 10
		case IntentGenerateQuestions, IntentGenerateVariants:
			params.Count = 3
		}
	}
	if params.Count > DefaultMaxGenerate {
		params.Count = DefaultMaxGenerate
	}
	if params.Count < 0 {
		params.Count = 0
	}
	plan := AgentPlan{
		Intent:  intent,
		Title:   title,
		Params:  params,
		Steps:   plannedSteps(intent),
		Message: message,
	}
	return plan, nil
}

func (s *Service) ExecuteAgent(ctx context.Context, principal auth.Principal, request AgentRequest) (AgentRun, error) {
	if err := validatePrincipal(principal); err != nil {
		return AgentRun{}, err
	}
	plan, err := PlanGoal(request)
	if err != nil {
		return AgentRun{}, err
	}
	runID := uuid.New()
	run := AgentRun{
		ID:       runID.String(),
		Goal:     request.Goal,
		Intent:   plan.Intent,
		Status:   stepSucceeded,
		Provider: s.currentProvider(),
		Model:    s.llm.Model(),
		Plan:     plan.Steps,
		Result:   map[string]any{"title": plan.Title},
	}
	if plan.Intent == IntentUnknown {
		run.Status = stepFailed
		run.Error = plan.Message
		run.Steps = []Step{{
			Key: "plan", Title: "解析目标", Status: stepFailed,
			Summary: plan.Message,
		}}
		return s.saveRun(ctx, principal, run)
	}
	steps, result, execErr := s.executePlan(ctx, principal, plan, runID.String())
	run.Steps = steps
	for key, value := range result {
		run.Result[key] = value
	}
	if execErr != nil {
		run.Status = stepFailed
		run.Error = execErr.Error()
	}
	saved, saveErr := s.saveRun(ctx, principal, run)
	if execErr != nil {
		return saved, nil
	}
	if saveErr != nil {
		return run, saveErr
	}
	return saved, nil
}

func (s *Service) executePlan(ctx context.Context, principal auth.Principal, plan AgentPlan, runID string) ([]Step, map[string]any, error) {
	result := map[string]any{
		"intent":   plan.Intent,
		"params":   plan.Params,
		"warnings": []string{},
	}
	steps := make([]Step, 0, len(plan.Steps)+1)
	steps = append(steps, Step{
		Key: "plan", Title: "解析目标", Status: stepSucceeded,
		Summary: plan.Title,
	})
	warnings := []string{}
	appendWarning := func(message string) {
		warnings = uniqueStrings(append(warnings, message))
		result["warnings"] = warnings
	}
	switch plan.Intent {
	case IntentAuditBank:
		step, audit, err := s.agentAudit(ctx, principal, plan.Params)
		steps = append(steps, step)
		result["audit"] = audit
		if err != nil {
			return steps, result, err
		}
	case IntentTagQuestions:
		step, ids, stepWarnings, err := s.agentTag(ctx, principal, plan.Params, runID)
		steps = append(steps, step)
		for _, warning := range stepWarnings {
			appendWarning(warning)
		}
		result["question_ids"] = ids
		if err != nil {
			return steps, result, err
		}
	case IntentFillExplanations:
		step, ids, stepWarnings, err := s.agentFill(ctx, principal, plan.Params, runID)
		steps = append(steps, step)
		for _, warning := range stepWarnings {
			appendWarning(warning)
		}
		result["question_ids"] = ids
		if err != nil {
			return steps, result, err
		}
	case IntentOrganizeBank:
		tagStep, tagIDs, tagWarnings, err := s.agentTag(ctx, principal, plan.Params, runID)
		steps = append(steps, tagStep)
		for _, warning := range tagWarnings {
			appendWarning(warning)
		}
		if err != nil {
			return steps, result, err
		}
		fillStep, fillIDs, fillWarnings, err := s.agentFill(ctx, principal, plan.Params, runID)
		steps = append(steps, fillStep)
		for _, warning := range fillWarnings {
			appendWarning(warning)
		}
		result["question_ids"] = uniqueStrings(append(tagIDs, fillIDs...))
		if err != nil {
			return steps, result, err
		}
	case IntentGenerateQuestions:
		step, generateResult, stepWarnings, err := s.agentGenerate(ctx, principal, plan.Params, runID)
		steps = append(steps, step)
		for _, warning := range stepWarnings {
			appendWarning(warning)
		}
		if generateResult != nil {
			result["question_ids"] = generateResult.QuestionIDs
			result["provider"] = generateResult.Provider
		}
		if err != nil {
			return steps, result, err
		}
	case IntentGenerateVariants:
		step, variantResult, stepWarnings, err := s.agentVariants(ctx, principal, plan.Params, runID)
		steps = append(steps, step)
		for _, warning := range stepWarnings {
			appendWarning(warning)
		}
		if variantResult != nil {
			result["question_ids"] = variantResult.QuestionIDs
			result["provider"] = variantResult.Provider
		}
		if err != nil {
			return steps, result, err
		}
	case IntentComposePaper:
		step, composeResult, stepWarnings, err := s.agentComposePaper(ctx, principal, plan.Params, runID)
		steps = append(steps, step)
		for _, warning := range stepWarnings {
			appendWarning(warning)
		}
		for key, value := range composeResult {
			result[key] = value
		}
		if err != nil {
			return steps, result, err
		}
	case IntentAnalyzeQuestion:
		step, analysisResult, stepWarnings, err := s.agentAnalyze(ctx, principal, plan.Params, runID)
		steps = append(steps, step)
		for _, warning := range stepWarnings {
			appendWarning(warning)
		}
		for key, value := range analysisResult {
			result[key] = value
		}
		if err != nil {
			return steps, result, err
		}
	default:
		return steps, result, fmt.Errorf("暂不支持该目标")
	}
	result["warnings"] = warnings
	return steps, result, nil
}

func (s *Service) agentAudit(ctx context.Context, principal auth.Principal, params AgentParams) (Step, AuditResult, error) {
	step := Step{Key: "audit", Title: "题库体检", Status: stepSucceeded}
	audit, err := s.AuditBank(ctx, principal, 1000)
	if err != nil {
		step.Status = stepFailed
		step.Warning = "题库体检失败"
		return step, AuditResult{}, err
	}
	step.Summary = fmt.Sprintf("共检查 %d 道题，发现 %d 组重复题、%d 道缺解析、%d 道缺知识点",
		audit.Total, audit.DuplicateCount, audit.MissingExplanation, audit.MissingKnowledgePoints)
	return step, audit, nil
}

func (s *Service) agentTag(ctx context.Context, principal auth.Principal, params AgentParams, runID string) (Step, []string, []string, error) {
	step := Step{Key: "tag", Title: "批量整理分类与知识点", Status: stepSucceeded}
	items, err := s.questions.Snapshot(ctx, principal, 1000)
	if err != nil {
		step.Status = stepFailed
		return step, nil, nil, err
	}
	selected := selectDrafts(items, principal, func(item question.Question) bool {
		return item.Category == "" || len(item.KnowledgePoints) == 0 || len(item.Tags) == 0
	}, DefaultMaxBatch)
	if len(selected) == 0 {
		step.Status = stepSkipped
		step.Summary = "没有需要整理分类/知识点的草稿"
		return step, nil, nil, nil
	}
	warnings := []string{}
	changed := []string{}
	for _, item := range selected {
		analysis, err := s.Analyze(ctx, principal, analyzeInputFromQuestion(item))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("题目 %s 分析失败", item.ID))
			continue
		}
		patch := buildMetadataPatch(item, analysis)
		if _, err := s.questions.ApplyAI(ctx, principal, item.ID, patch); err != nil {
			warnings = append(warnings, fmt.Sprintf("题目 %s 写入失败", item.ID))
			continue
		}
		changed = append(changed, item.ID.String())
		if s.repo != nil {
			_ = s.repo.CreateAudit(ctx, principal, AuditLog{
				Action: "agent_tag_question", TargetType: "question", TargetID: item.ID.String(), RunID: runID,
				Detail: map[string]any{
					"provider":         analysis.Provider,
					"category":         analysis.Category,
					"knowledge_points": analysis.KnowledgePoints,
				},
			})
		}
	}
	step.Affected = len(changed)
	step.QuestionIDs = changed
	step.Summary = fmt.Sprintf("已整理 %d 道草稿的分类、知识点与标签", len(changed))
	if len(warnings) > 0 {
		step.Warning = fmt.Sprintf("%d 道题处理失败", len(warnings))
	}
	return step, changed, warnings, nil
}

func (s *Service) agentFill(ctx context.Context, principal auth.Principal, params AgentParams, runID string) (Step, []string, []string, error) {
	step := Step{Key: "fill", Title: "批量补答案与解析", Status: stepSucceeded}
	items, err := s.questions.Snapshot(ctx, principal, 1000)
	if err != nil {
		step.Status = stepFailed
		return step, nil, nil, err
	}
	selected := selectDrafts(items, principal, func(item question.Question) bool {
		return strings.TrimSpace(item.Explanation) == "" || strings.TrimSpace(item.Answer) == ""
	}, DefaultMaxBatch)
	if len(selected) == 0 {
		step.Status = stepSkipped
		step.Summary = "没有缺少答案或解析的草稿"
		return step, nil, nil, nil
	}
	warnings := []string{}
	changed := []string{}
	for _, item := range selected {
		analysis, err := s.Analyze(ctx, principal, analyzeInputFromQuestion(item))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("题目 %s 分析失败", item.ID))
			continue
		}
		patch := buildFillPatch(item, analysis)
		if _, err := s.questions.ApplyAI(ctx, principal, item.ID, patch); err != nil {
			warnings = append(warnings, fmt.Sprintf("题目 %s 写入失败", item.ID))
			continue
		}
		changed = append(changed, item.ID.String())
		if s.repo != nil {
			_ = s.repo.CreateAudit(ctx, principal, AuditLog{
				Action: "agent_fill_explanation", TargetType: "question", TargetID: item.ID.String(), RunID: runID,
				Detail: map[string]any{
					"provider":   analysis.Provider,
					"has_answer": strings.TrimSpace(item.Answer) != "" || strings.TrimSpace(analysis.Answer) != "",
				},
			})
		}
	}
	step.Affected = len(changed)
	step.QuestionIDs = changed
	step.Summary = fmt.Sprintf("已为 %d 道草稿补充答案或解析", len(changed))
	if len(warnings) > 0 {
		step.Warning = fmt.Sprintf("%d 道题处理失败", len(warnings))
	}
	return step, changed, warnings, nil
}

func (s *Service) agentGenerate(ctx context.Context, principal auth.Principal, params AgentParams, runID string) (Step, *GenerateResult, []string, error) {
	step := Step{Key: "generate", Title: "按知识点生成新题", Status: stepSucceeded}
	subject := params.Subject
	if principal.Role == auth.RoleTeacher {
		subject = principal.Subject
	}
	if subject == "" {
		step.Status = stepFailed
		return step, nil, nil, fmt.Errorf("请在目标中说明学科")
	}
	if params.Grade == "" {
		step.Status = stepFailed
		return step, nil, nil, fmt.Errorf("请在目标中说明年级")
	}
	count := params.Count
	if count <= 0 {
		count = 3
	}
	result, err := s.generate(ctx, principal, GenerateInput{
		Subject:         subject,
		Grade:           params.Grade,
		Category:        params.Category,
		KnowledgePoints: splitKnowledge(params.KnowledgePoint),
		Tags:            splitKnowledge(params.Tag),
		Count:           count,
		Reference:       params.KnowledgePoint,
		Persist:         true,
	}, runID)
	if err != nil {
		step.Status = stepFailed
		step.Warning = "生成题目失败"
		return step, nil, nil, err
	}
	step.Affected = len(result.QuestionIDs)
	step.QuestionIDs = result.QuestionIDs
	step.Summary = fmt.Sprintf("已生成并保存 %d 道草稿", len(result.QuestionIDs))
	if len(result.Warnings) > 0 {
		step.Warning = strings.Join(result.Warnings, "；")
	}
	return step, &result, result.Warnings, nil
}

func (s *Service) agentVariants(ctx context.Context, principal auth.Principal, params AgentParams, runID string) (Step, *VariantResult, []string, error) {
	step := Step{Key: "variants", Title: "生成题目变式", Status: stepSucceeded}
	source, err := s.findVariantSource(ctx, principal, params)
	if err != nil {
		step.Status = stepFailed
		step.Warning = err.Error()
		return step, nil, nil, err
	}
	if source == nil {
		step.Status = stepFailed
		err := fmt.Errorf("没有找到可用于生成变式的题目，请先在目标中提供题目ID")
		step.Warning = err.Error()
		return step, nil, nil, err
	}
	count := params.Count
	if count <= 0 {
		count = 3
	}
	result, err := s.generateVariants(ctx, principal, source.ID, VariantInput{
		Count: count, Strategy: "numbers", Persist: true,
	}, runID)
	if err != nil {
		step.Status = stepFailed
		step.Warning = "生成变式失败"
		return step, nil, nil, err
	}
	step.Affected = len(result.QuestionIDs)
	step.QuestionIDs = result.QuestionIDs
	step.Summary = fmt.Sprintf("基于《%s》生成并保存 %d 道变式草稿", truncate(plainText(source.Content), 20), len(result.QuestionIDs))
	if len(result.Warnings) > 0 {
		step.Warning = strings.Join(result.Warnings, "；")
	}
	return step, &result, result.Warnings, nil
}

func (s *Service) agentComposePaper(ctx context.Context, principal auth.Principal, params AgentParams, runID string) (Step, map[string]any, []string, error) {
	step := Step{Key: "compose", Title: "智能组卷", Status: stepSucceeded}
	result := map[string]any{}
	if s.papers == nil {
		step.Status = stepFailed
		return step, result, nil, fmt.Errorf("组卷服务不可用")
	}
	subject := params.Subject
	if principal.Role == auth.RoleTeacher {
		subject = principal.Subject
	}
	if subject == "" {
		step.Status = stepFailed
		return step, result, nil, fmt.Errorf("请在目标中说明学科")
	}
	if params.Grade == "" {
		step.Status = stepFailed
		return step, result, nil, fmt.Errorf("请在目标中说明年级")
	}
	items, err := s.questions.Snapshot(ctx, principal, 1000)
	if err != nil {
		step.Status = stepFailed
		return step, result, nil, err
	}
	pool := make([]question.Question, 0)
	for _, item := range items {
		if item.Status != question.StatusPublished {
			continue
		}
		if item.Subject != subject || (params.Grade != "" && item.Grade != params.Grade) {
			continue
		}
		if params.Category != "" && item.Category != params.Category {
			continue
		}
		if params.KnowledgePoint != "" && !containsString(item.KnowledgePoints, params.KnowledgePoint) {
			continue
		}
		if params.Tag != "" && !containsString(item.Tags, params.Tag) {
			continue
		}
		pool = append(pool, item)
	}
	if len(pool) == 0 {
		step.Status = stepFailed
		return step, result, nil, fmt.Errorf("没有符合条件的已发布题目")
	}
	count := params.Count
	if count <= 0 {
		count = 10
	}
	if count > len(pool) {
		count = len(pool)
	}
	selected := balanceByDifficulty(pool, count)
	paperQuestions := make([]paper.QuestionInput, 0, len(selected))
	total := 100.0
	per := math.Round(total/float64(len(selected))*100) / 100
	remaining := total
	for index, item := range selected {
		score := per
		if index == len(selected)-1 {
			score = math.Round(remaining*100) / 100
		}
		remaining = math.Round((remaining-score)*100) / 100
		paperQuestions = append(paperQuestions, paper.QuestionInput{
			QuestionID:    item.ID,
			AssignedScore: strconv.FormatFloat(score, 'f', 2, 64),
		})
	}
	title := fmt.Sprintf("AI 组卷 · %s%s · %s", subject, params.Grade, time.Now().Format("2006-01-02"))
	duration := len(selected) * 8
	if duration < 30 {
		duration = 30
	}
	if duration > 120 {
		duration = 120
	}
	created, err := s.papers.Create(ctx, principal, paper.Input{
		Title:           title,
		Subject:         subject,
		Grade:           params.Grade,
		DurationMinutes: duration,
		Questions:       paperQuestions,
	})
	if err != nil {
		step.Status = stepFailed
		step.Warning = "组卷失败"
		return step, result, nil, err
	}
	ids := make([]string, 0, len(selected))
	for _, item := range selected {
		ids = append(ids, item.ID.String())
	}
	result["paper_id"] = created.ID.String()
	result["paper_title"] = created.Title
	result["question_ids"] = ids
	step.Affected = len(selected)
	step.QuestionIDs = ids
	step.Summary = fmt.Sprintf("已生成试卷《%s》，共 %d 题", created.Title, len(selected))
	if s.repo != nil {
		_ = s.repo.CreateAudit(ctx, principal, AuditLog{
			Action: "agent_compose_paper", TargetType: "paper", TargetID: created.ID.String(), RunID: runID,
			Detail: map[string]any{
				"subject": subject, "grade": params.Grade, "question_count": len(selected),
			},
		})
	}
	return step, result, nil, nil
}

func (s *Service) agentAnalyze(ctx context.Context, principal auth.Principal, params AgentParams, runID string) (Step, map[string]any, []string, error) {
	step := Step{Key: "analyze", Title: "分析题目", Status: stepSucceeded}
	result := map[string]any{}
	if params.QuestionID == "" {
		step.Status = stepFailed
		return step, result, nil, fmt.Errorf("请提供要分析的题目ID")
	}
	id, err := uuid.Parse(params.QuestionID)
	if err != nil {
		step.Status = stepFailed
		return step, result, nil, fmt.Errorf("题目ID格式不正确")
	}
	item, err := s.questions.Get(ctx, principal, id)
	if err != nil {
		step.Status = stepFailed
		return step, result, nil, err
	}
	apply := item.Status == question.StatusDraft
	if principal.Role == auth.RoleTeacher && item.CreatorID != principal.UserID {
		apply = false
	}
	analysisResult, err := s.analyzeQuestion(ctx, principal, id, apply, runID)
	if err != nil {
		step.Status = stepFailed
		return step, result, nil, err
	}
	result["analysis"] = analysisResult.Analysis
	result["applied"] = analysisResult.Applied
	result["question_id"] = id.String()
	step.Affected = 1
	step.QuestionIDs = []string{id.String()}
	if analysisResult.Applied {
		step.Summary = "已完成分析并写回草稿"
	} else {
		step.Summary = "已完成分析；题目非本人草稿或已发布，未自动修改"
	}
	step.Warning = strings.Join(analysisResult.Analysis.Warnings, "；")
	return step, result, analysisResult.Analysis.Warnings, nil
}

func (s *Service) findVariantSource(ctx context.Context, principal auth.Principal, params AgentParams) (*question.Question, error) {
	if params.QuestionID != "" {
		id, err := uuid.Parse(params.QuestionID)
		if err != nil {
			return nil, fmt.Errorf("题目ID格式不正确")
		}
		item, err := s.questions.Get(ctx, principal, id)
		if err != nil {
			return nil, err
		}
		return &item, nil
	}
	items, err := s.questions.Snapshot(ctx, principal, 200)
	if err != nil {
		return nil, err
	}
	subject := params.Subject
	if principal.Role == auth.RoleTeacher {
		subject = principal.Subject
	}
	for index := range items {
		item := items[index]
		if item.Status != question.StatusDraft {
			continue
		}
		if subject != "" && item.Subject != subject {
			continue
		}
		if params.Grade != "" && item.Grade != params.Grade {
			continue
		}
		if params.KnowledgePoint != "" && !containsString(item.KnowledgePoints, params.KnowledgePoint) {
			continue
		}
		return &item, nil
	}
	return nil, nil
}

func (s *Service) saveRun(ctx context.Context, principal auth.Principal, run AgentRun) (AgentRun, error) {
	if s.repo == nil {
		return run, nil
	}
	saved, err := s.repo.CreateRun(ctx, principal, run)
	if err != nil {
		return run, err
	}
	return saved, nil
}

func (s *Service) currentProvider() string {
	if s.llm.Enabled() {
		return ProviderLLM
	}
	return ProviderLocal
}

func selectDrafts(items []question.Question, principal auth.Principal, needs func(question.Question) bool, limit int) []question.Question {
	selected := make([]question.Question, 0, limit)
	for _, item := range items {
		if len(selected) >= limit {
			break
		}
		if item.Status != question.StatusDraft {
			continue
		}
		if principal.Role == auth.RoleTeacher && item.CreatorID != principal.UserID {
			continue
		}
		if !needs(item) {
			continue
		}
		selected = append(selected, item)
	}
	return selected
}

func buildMetadataPatch(existing question.Question, analysis Analysis) question.AIApplyPatch {
	patch := question.AIApplyPatch{
		Provider:       analysis.Provider,
		Model:          analysis.Model,
		Confidence:     analysis.Confidence,
		Rationale:      analysis.Rationale,
		ReviewRequired: true,
	}
	if analysis.Category != "" {
		patch.Category = &analysis.Category
	}
	if len(analysis.KnowledgePoints) > 0 {
		values := uniqueStrings(analysis.KnowledgePoints)
		patch.KnowledgePoints = &values
	}
	if len(analysis.Tags) > 0 {
		values := uniqueStrings(append(append([]string{}, existing.Tags...), analysis.Tags...))
		if len(values) > 10 {
			values = values[:10]
		}
		patch.Tags = &values
	}
	if analysis.Difficulty >= 1 && analysis.Difficulty <= 5 {
		value := analysis.Difficulty
		patch.Difficulty = &value
	}
	return patch
}

func buildFillPatch(existing question.Question, analysis Analysis) question.AIApplyPatch {
	patch := question.AIApplyPatch{
		Provider:       analysis.Provider,
		Model:          analysis.Model,
		Confidence:     analysis.Confidence,
		Rationale:      analysis.Rationale,
		ReviewRequired: true,
	}
	if strings.TrimSpace(existing.Answer) == "" && strings.TrimSpace(analysis.Answer) != "" {
		if value, err := question.ValidateAnswer(existing.QuestionType, existing.Options, analysis.Answer); err == nil {
			patch.Answer = &value
		}
	}
	if strings.TrimSpace(existing.Explanation) == "" && strings.TrimSpace(analysis.Explanation) != "" {
		value := analysis.Explanation
		patch.Explanation = &value
	}
	return patch
}

func plannedSteps(intent string) []Step {
	titles := map[string][]string{
		IntentAuditBank:         {"读取题库快照", "检查完整性与重复题", "汇总体检报告"},
		IntentTagQuestions:      {"读取待整理草稿", "分析分类与知识点", "写回草稿元数据"},
		IntentFillExplanations:  {"读取缺少答案/解析的草稿", "生成答案与解析", "写回草稿"},
		IntentOrganizeBank:      {"读取待整理草稿", "补齐分类与知识点", "补齐答案与解析", "汇总变更"},
		IntentGenerateQuestions: {"解析出题条件", "生成题目", "保存为草稿"},
		IntentGenerateVariants:  {"定位原题", "生成变式", "保存为草稿"},
		IntentComposePaper:      {"筛选已发布题目", "按难度配平", "生成试卷草稿"},
		IntentAnalyzeQuestion:   {"读取题目", "分析题目", "按权限写回草稿"},
	}
	steps := make([]Step, 0, len(titles[intent]))
	for index, title := range titles[intent] {
		steps = append(steps, Step{
			Key: fmt.Sprintf("planned_%d", index+1), Title: title, Status: stepPlanned,
		})
	}
	return steps
}

func detectIntent(request AgentRequest) (string, string, string) {
	goal := strings.ToLower(request.Goal)
	has := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(goal, strings.ToLower(word)) {
				return true
			}
		}
		return false
	}
	switch {
	case request.QuestionID != "" && has("分析", "看看", "诊断"):
		return IntentAnalyzeQuestion, "分析指定题目", ""
	case has("组卷", "试卷", "出一份卷", "出一套卷"):
		return IntentComposePaper, "按条件智能组卷", ""
	case has("变式", "改编", "类似题", "举一反三", "同类型"):
		return IntentGenerateVariants, "生成题目变式", ""
	case has("出题", "生成题", "生成题目", "新题", "命题", "编题"):
		return IntentGenerateQuestions, "按知识点出题", ""
	case has("解析", "答案", "补全") && has("标签", "分类", "知识点", "整理", "打标"):
		return IntentOrganizeBank, "整理题库并补齐解析", ""
	case has("解析", "答案", "补全"):
		return IntentFillExplanations, "批量补充答案与解析", ""
	case has("标签", "分类", "知识点", "整理", "打标", "归类"):
		return IntentTagQuestions, "批量整理分类与知识点", ""
	case has("体检", "检查", "审计", "质量", "重复", "统计", "分析题库"):
		return IntentAuditBank, "题库质量体检", ""
	case has("分析"):
		return IntentUnknown, "需要更明确的目标", "请提供题目ID，或说明要分析整个题库还是生成题目"
	default:
		return IntentUnknown, "暂不支持该目标", "可以尝试：题库体检、批量补解析、整理分类/知识点、生成变式、按知识点出题、智能组卷"
	}
}

func extractCount(goal string) int {
	if match := agentCountPattern.FindStringSubmatch(goal); len(match) == 2 {
		if value, err := strconv.Atoi(match[1]); err == nil {
			return value
		}
	}
	if match := agentChineseCountPattern.FindStringSubmatch(goal); len(match) == 2 {
		return chineseNumber[match[1]]
	}
	return 0
}

func extractQuoted(goal string) string {
	if match := agentQuotedPattern.FindStringSubmatch(goal); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func matchKnowledgePoint(goal string) string {
	best := ""
	for _, dictionary := range knowledgeDictionary {
		for _, keyword := range dictionary {
			if strings.Contains(goal, keyword) && len([]rune(keyword)) > len([]rune(best)) {
				best = keyword
			}
		}
	}
	return best
}

func splitKnowledge(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	})
	return uniqueStrings(fields)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func balanceByDifficulty(items []question.Question, count int) []question.Question {
	buckets := map[int][]question.Question{}
	for _, item := range items {
		buckets[item.Difficulty] = append(buckets[item.Difficulty], item)
	}
	selected := make([]question.Question, 0, count)
	for len(selected) < count {
		added := false
		for difficulty := 1; difficulty <= 5 && len(selected) < count; difficulty++ {
			if len(buckets[difficulty]) == 0 {
				continue
			}
			selected = append(selected, buckets[difficulty][0])
			buckets[difficulty] = buckets[difficulty][1:]
			added = true
		}
		if !added {
			break
		}
	}
	return selected
}
