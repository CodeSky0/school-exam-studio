package ai

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/paper"
)

func (s *Service) toolListPapers(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.papers == nil {
		return ToolResult{}, fmt.Errorf("试卷服务不可用")
	}
	limit := intArgument(args, "limit", 20, 1, 50)
	filter := paper.PaperFilter{
		Subject:  stringArgument(args, "subject"),
		Grade:    stringArgument(args, "grade"),
		Page:     1,
		PageSize: 50,
	}
	if toolContext.Principal.Role == auth.RoleTeacher {
		filter.Subject = toolContext.Principal.Subject
	}
	result, err := s.papers.List(ctx, toolContext.Principal, filter)
	if err != nil {
		return ToolResult{}, err
	}
	keyword := strings.ToLower(strings.TrimSpace(stringArgument(args, "keyword")))
	items := make([]map[string]any, 0, limit)
	for _, item := range result.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Title), keyword) {
			continue
		}
		items = append(items, map[string]any{
			"paper_id":         item.ID.String(),
			"title":            item.Title,
			"subject":          item.Subject,
			"grade":            item.Grade,
			"question_count":   item.QuestionCount,
			"total_score":      item.TotalScore,
			"duration_minutes": item.DurationMinutes,
			"updated_at":       item.UpdatedAt,
		})
		if len(items) >= limit {
			break
		}
	}
	return ToolResult{
		Summary: fmt.Sprintf("找到 %d 份试卷（共 %d 份）", len(items), result.Total),
		Data:    map[string]any{"items": items, "total": result.Total},
	}, nil
}

func (s *Service) toolGetPaper(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.papers == nil {
		return ToolResult{}, fmt.Errorf("试卷服务不可用")
	}
	id, err := uuidArgument(args, "paper_id")
	if err != nil {
		return ToolResult{}, err
	}
	item, err := s.papers.Get(ctx, toolContext.Principal, id)
	if err != nil {
		return ToolResult{}, err
	}
	questions := make([]map[string]any, 0, len(item.Questions))
	for _, entry := range item.Questions {
		questions = append(questions, map[string]any{
			"question_id":    entry.QuestionID.String(),
			"order_index":    entry.OrderIndex,
			"assigned_score": entry.AssignedScore,
			"question":       toSnapshot(entry.Question),
		})
	}
	return ToolResult{
		Summary: fmt.Sprintf("已读取试卷《%s》，共 %d 道题", truncate(item.Title, 24), len(item.Questions)),
		Data: map[string]any{
			"paper_id":         item.ID.String(),
			"title":            item.Title,
			"subject":          item.Subject,
			"grade":            item.Grade,
			"total_score":      item.TotalScore,
			"duration_minutes": item.DurationMinutes,
			"questions":        questions,
		},
	}, nil
}

func (s *Service) toolPaperAudit(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.papers == nil {
		return ToolResult{}, fmt.Errorf("试卷服务不可用")
	}
	id, err := uuidArgument(args, "paper_id")
	if err != nil {
		return ToolResult{}, err
	}
	item, err := s.papers.Get(ctx, toolContext.Principal, id)
	if err != nil {
		return ToolResult{}, err
	}
	totalScore := 0.0
	if parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(item.TotalScore), 64); parseErr == nil {
		totalScore = parsed
	}
	difficulty := map[string]int{}
	knowledge := map[string]int{}
	missingAnswer := make([]string, 0)
	missingExplanation := make([]string, 0)
	for _, entry := range item.Questions {
		snapshot := entry.Question
		difficulty[strconv.Itoa(snapshot.Difficulty)]++
		for _, point := range snapshot.KnowledgePoints {
			knowledge[point]++
		}
		if strings.TrimSpace(snapshot.Answer) == "" {
			missingAnswer = append(missingAnswer, snapshot.ID.String())
		}
		if strings.TrimSpace(snapshot.Explanation) == "" {
			missingExplanation = append(missingExplanation, snapshot.ID.String())
		}
	}
	warnings := []string{}
	if len(missingAnswer) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d 道题缺少答案", len(missingAnswer)))
	}
	if len(missingExplanation) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d 道题缺少解析", len(missingExplanation)))
	}
	return ToolResult{
		Summary: fmt.Sprintf("试卷《%s》共 %d 道题，总分 %s，缺答案 %d 道，缺解析 %d 道",
			truncate(item.Title, 24), len(item.Questions), item.TotalScore, len(missingAnswer), len(missingExplanation)),
		Data: map[string]any{
			"paper_id":                      item.ID.String(),
			"title":                         item.Title,
			"question_count":                len(item.Questions),
			"total_score":                   totalScore,
			"difficulty_distribution":       difficulty,
			"knowledge_points":              knowledge,
			"missing_answer_questions":      missingAnswer,
			"missing_explanation_questions": missingExplanation,
		},
		Warnings: warnings,
	}, nil
}

func (s *Service) toolUpdatePaperDraft(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.papers == nil {
		return ToolResult{}, fmt.Errorf("试卷服务不可用")
	}
	id, err := uuidArgument(args, "paper_id")
	if err != nil {
		return ToolResult{}, err
	}
	current, err := s.papers.Get(ctx, toolContext.Principal, id)
	if err != nil {
		return ToolResult{}, err
	}
	if toolContext.Principal.Role != auth.RoleAdmin && current.CreatorID != toolContext.Principal.UserID {
		return ToolResult{}, validationError("paper_id", "只能修改本人创建的试卷")
	}
	rawQuestions, ok := args["questions"].([]any)
	if !ok || len(rawQuestions) == 0 {
		return ToolResult{}, validationError("questions", "至少需要一道题目")
	}
	if len(rawQuestions) > 100 {
		return ToolResult{}, validationError("questions", "单份试卷最多 100 道题")
	}
	questions := make([]paper.QuestionInput, 0, len(rawQuestions))
	for index, raw := range rawQuestions {
		entry, ok := raw.(map[string]any)
		if !ok {
			return ToolResult{}, validationError(fmt.Sprintf("questions[%d]", index), "题目必须是对象")
		}
		questionID, parseErr := uuidArgument(entry, "question_id")
		if parseErr != nil {
			return ToolResult{}, validationError(fmt.Sprintf("questions[%d].question_id", index), "题目 ID 无效")
		}
		score := assignedScoreArgument(entry)
		if score == "" {
			return ToolResult{}, validationError(fmt.Sprintf("questions[%d].assigned_score", index), "分值必须是正数")
		}
		questions = append(questions, paper.QuestionInput{QuestionID: questionID, AssignedScore: score})
	}
	input := paper.Input{
		Title:           firstNonEmpty(stringArgument(args, "title"), current.Title),
		Subject:         current.Subject,
		Grade:           current.Grade,
		DurationMinutes: intArgument(args, "duration_minutes", current.DurationMinutes, 1, 600),
		Questions:       questions,
	}
	updated, err := s.papers.Update(ctx, toolContext.Principal, id, input)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Summary: fmt.Sprintf("已更新试卷《%s》：%d 道题，总分 %s", truncate(updated.Title, 24), len(questions), updated.TotalScore),
		Data: map[string]any{
			"paper_id":       updated.ID.String(),
			"title":          updated.Title,
			"total_score":    updated.TotalScore,
			"question_count": len(questions),
		},
		Warnings: []string{"试卷修改仅作用于草稿，请在发布前人工复核"},
	}, nil
}

func assignedScoreArgument(entry map[string]any) string {
	switch value := entry["assigned_score"].(type) {
	case float64:
		if value > 0 {
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	case string:
		trimmed := strings.TrimSpace(value)
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil && parsed > 0 {
			return trimmed
		}
	}
	return ""
}
