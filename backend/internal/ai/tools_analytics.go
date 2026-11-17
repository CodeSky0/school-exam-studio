package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/question"
)

func (s *Service) toolBankStats(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	limit := intArgument(args, "limit", 1000, 1, 1000)
	items, err := s.questions.Snapshot(ctx, toolContext.Principal, limit)
	if err != nil {
		return ToolResult{}, err
	}
	subjectFilter := stringArgument(args, "subject")
	gradeFilter := stringArgument(args, "grade")
	bySubject := map[string]int{}
	byGrade := map[string]int{}
	byType := map[string]int{}
	byDifficulty := map[string]int{}
	byKnowledge := map[string]int{}
	aiGenerated := 0
	total := 0
	for _, item := range items {
		if subjectFilter != "" && item.Subject != subjectFilter {
			continue
		}
		if gradeFilter != "" && item.Grade != gradeFilter {
			continue
		}
		total++
		bySubject[item.Subject]++
		byGrade[item.Grade]++
		byType[item.QuestionType]++
		byDifficulty[fmt.Sprintf("%d", item.Difficulty)]++
		for _, point := range item.KnowledgePoints {
			byKnowledge[point]++
		}
		if item.AIGenerated {
			aiGenerated++
		}
	}
	sparse := knowledgeGaps(byKnowledge)
	return ToolResult{
		Summary: fmt.Sprintf("题库共 %d 道题，覆盖 %d 个知识点，其中 %d 个知识点仅 1 道题", total, len(byKnowledge), len(sparse)),
		Data: map[string]any{
			"total":              total,
			"by_subject":         bySubject,
			"by_grade":           byGrade,
			"by_type":            byType,
			"by_difficulty":      byDifficulty,
			"knowledge_points":   byKnowledge,
			"sparse_knowledge":   sparse,
			"ai_generated_count": aiGenerated,
		},
		Warnings: []string{"统计基于当前账号可见的最近 1000 道题"},
	}, nil
}

func knowledgeGaps(distribution map[string]int) []map[string]any {
	gaps := make([]map[string]any, 0)
	for point, count := range distribution {
		if count <= 1 {
			gaps = append(gaps, map[string]any{"knowledge_point": point, "count": count})
		}
	}
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i]["knowledge_point"].(string) < gaps[j]["knowledge_point"].(string)
	})
	if len(gaps) > 20 {
		gaps = gaps[:20]
	}
	return gaps
}

func (s *Service) toolFindDuplicates(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	limit := intArgument(args, "limit", 500, 1, 1000)
	items, err := s.questions.Snapshot(ctx, toolContext.Principal, limit)
	if err != nil {
		return ToolResult{}, err
	}
	snapshots := make([]QuestionSnapshot, 0, len(items))
	for _, item := range items {
		snapshots = append(snapshots, toSnapshot(item))
	}
	audit := s.local.Audit(snapshots)
	groups := make([]map[string]any, 0, len(audit.Duplicates))
	for _, group := range audit.Duplicates {
		groups = append(groups, map[string]any{
			"content_hash":    group.ContentHash,
			"question_ids":    group.QuestionIDs,
			"duplicate_count": len(group.QuestionIDs),
			"preview":         group.Preview,
		})
	}
	return ToolResult{
		Summary: fmt.Sprintf("体检 %d 道题，发现 %d 组重复题", audit.Total, len(groups)),
		Data: map[string]any{
			"total":           audit.Total,
			"groups":          groups,
			"duplicate_count": len(groups),
		},
	}, nil
}

func (s *Service) toolReviewQueue(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	limit := intArgument(args, "limit", 50, 1, 100)
	subjectFilter := stringArgument(args, "subject")
	gradeFilter := stringArgument(args, "grade")
	items, err := s.questions.Snapshot(ctx, toolContext.Principal, 1000)
	if err != nil {
		return ToolResult{}, err
	}
	pending := make([]map[string]any, 0, limit)
	for _, item := range items {
		if item.Status != question.StatusDraft || !item.AIGenerated {
			continue
		}
		if !reviewRequired(item) {
			continue
		}
		if subjectFilter != "" && item.Subject != subjectFilter {
			continue
		}
		if gradeFilter != "" && item.Grade != gradeFilter {
			continue
		}
		pending = append(pending, map[string]any{
			"question_id":      item.ID.String(),
			"content_preview":  truncate(plainText(item.Content), 120),
			"subject":          item.Subject,
			"grade":            item.Grade,
			"question_type":    item.QuestionType,
			"difficulty":       item.Difficulty,
			"knowledge_points": item.KnowledgePoints,
			"updated_at":       item.UpdatedAt,
		})
		if len(pending) >= limit {
			break
		}
	}
	return ToolResult{
		Summary:  fmt.Sprintf("有 %d 道 AI 生成的草稿等待人工复核", len(pending)),
		Data:     map[string]any{"items": pending, "total": len(pending)},
		Warnings: []string{"复核通过前不要发布这些题目"},
	}, nil
}

func reviewRequired(item question.Question) bool {
	if !item.AIGenerated {
		return false
	}
	value, ok := item.AIMeta["review_required"]
	if !ok {
		return true
	}
	required, ok := value.(bool)
	if !ok {
		return true
	}
	return required
}

func (s *Service) toolSearchSimilar(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	limit := intArgument(args, "limit", 10, 1, 30)
	keyword := strings.TrimSpace(stringArgument(args, "keyword"))
	sourceID := ""
	if raw := strings.TrimSpace(stringArgument(args, "question_id")); raw != "" {
		id, err := uuidArgument(args, "question_id")
		if err != nil {
			return ToolResult{}, err
		}
		source, err := s.questions.Get(ctx, toolContext.Principal, id)
		if err != nil {
			return ToolResult{}, err
		}
		sourceID = source.ID.String()
		keyword = plainText(source.Content)
	}
	if keyword == "" {
		return ToolResult{}, validationError("keyword", "请提供题目 ID 或题干关键词")
	}
	items, err := s.questions.Snapshot(ctx, toolContext.Principal, 1000)
	if err != nil {
		return ToolResult{}, err
	}
	subjectFilter := stringArgument(args, "subject")
	gradeFilter := stringArgument(args, "grade")
	if toolContext.Principal.Role == auth.RoleTeacher {
		subjectFilter = toolContext.Principal.Subject
	}
	sourceTokens := textTokens(keyword)
	type scored struct {
		score float64
		item  question.Question
	}
	matches := make([]scored, 0)
	for _, item := range items {
		if item.ID.String() == sourceID {
			continue
		}
		if subjectFilter != "" && item.Subject != subjectFilter {
			continue
		}
		if gradeFilter != "" && item.Grade != gradeFilter {
			continue
		}
		score := overlapScore(sourceTokens, textTokens(plainText(item.Content)))
		if score <= 0 {
			continue
		}
		matches = append(matches, scored{score: score, item: item})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].item.ID.String() < matches[j].item.ID.String()
		}
		return matches[i].score > matches[j].score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	results := make([]map[string]any, 0, len(matches))
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.item.ID.String())
		results = append(results, map[string]any{
			"question_id":      match.item.ID.String(),
			"content_preview":  truncate(plainText(match.item.Content), 120),
			"subject":          match.item.Subject,
			"grade":            match.item.Grade,
			"question_type":    match.item.QuestionType,
			"difficulty":       match.item.Difficulty,
			"knowledge_points": match.item.KnowledgePoints,
			"score":            match.score,
		})
	}
	return ToolResult{
		Summary:     fmt.Sprintf("找到 %d 道相似题", len(results)),
		QuestionIDs: ids,
		Data:        map[string]any{"items": results, "total": len(results)},
	}, nil
}

// textTokens 使用字符二元组覆盖中文分词缺失的场景，英文与数字按词切分。
func textTokens(text string) map[string]struct{} {
	tokens := map[string]struct{}{}
	runes := []rune(strings.ToLower(text))
	for index := 0; index < len(runes)-1; index++ {
		left, right := runes[index], runes[index+1]
		if isWordRune(left) || isWordRune(right) {
			tokens[string([]rune{left, right})] = struct{}{}
		}
	}
	return tokens
}

func isWordRune(value rune) bool {
	return (value >= '0' && value <= '9') || (value >= 'a' && value <= 'z') || value >= 0x4e00
}

func overlapScore(left, right map[string]struct{}) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	shared := 0
	for token := range left {
		if _, ok := right[token]; ok {
			shared++
		}
	}
	if shared == 0 {
		return 0
	}
	union := len(left) + len(right) - shared
	return float64(shared) / float64(union)
}
