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
	IntentChat = "chat"

	stepSucceeded = "succeeded"
	stepFailed    = "failed"
	stepSkipped   = "skipped"
)

var (
	agentCountPattern        = regexp.MustCompile(`(\d{1,2})\s*(?:道|个|条|题)`)
	agentChineseCountPattern = regexp.MustCompile(`([一二三四五六七八九十])\s*(?:道|个|条|题)`)
	agentQuotedPattern       = regexp.MustCompile(`[「《“"]([^」》”"]{2,30})[」》”"]`)
)

var chineseNumber = map[string]int{
	"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
	"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
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
		input := analyzeInputFromQuestion(item)
		input.Model = params.Model
		analysis, err := s.Analyze(ctx, principal, input)
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
		input := analyzeInputFromQuestion(item)
		input.Model = params.Model
		analysis, err := s.Analyze(ctx, principal, input)
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
		Model:           params.Model,
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
		Count: count, Strategy: "numbers", Persist: true, Model: params.Model,
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
	if len(selected) == 0 {
		step.Status = stepFailed
		return step, result, nil, fmt.Errorf("没有可用于组卷的题目")
	}
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
	for _, subject := range subjectOrder {
		for _, keyword := range knowledgeDictionary[subject] {
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
	selectedIDs := make(map[uuid.UUID]struct{}, count)
	for len(selected) < count {
		added := false
		for difficulty := 1; difficulty <= 5 && len(selected) < count; difficulty++ {
			if len(buckets[difficulty]) == 0 {
				continue
			}
			selected = append(selected, buckets[difficulty][0])
			selectedIDs[buckets[difficulty][0].ID] = struct{}{}
			buckets[difficulty] = buckets[difficulty][1:]
			added = true
		}
		if !added {
			break
		}
	}
	// 难度不在 1-5 的历史数据不会进入配平桶，这里兜底补足，避免少题甚至除零。
	for _, item := range items {
		if len(selected) >= count {
			break
		}
		if _, ok := selectedIDs[item.ID]; ok {
			continue
		}
		selected = append(selected, item)
	}
	return selected
}
