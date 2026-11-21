package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/drive"
	"school-exam-studio/backend/internal/question"
)

// ToolDescriptor 是暴露给前端与外部 Agent 的工具契约。
type ToolDescriptor struct {
	Name        string          `json:"name"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Writes      bool            `json:"writes"`
	Category    string          `json:"category"`
}

type ToolContext struct {
	Principal      auth.Principal
	ConversationID string
	RunID          string
	AttachmentIDs  []string
}

type ToolResult struct {
	Summary     string         `json:"summary"`
	Data        map[string]any `json:"data,omitempty"`
	QuestionIDs []string       `json:"question_ids,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
}

type toolHandler func(context.Context, ToolContext, map[string]any) (ToolResult, error)

type registeredTool struct {
	descriptor ToolDescriptor
	run        toolHandler
}

func (s *Service) toolRegistry() map[string]registeredTool {
	tools := []registeredTool{
		{descriptor: toolSearchQuestions, run: s.toolSearchQuestions},
		{descriptor: toolGetQuestion, run: s.toolGetQuestion},
		{descriptor: toolBankAudit, run: s.toolBankAudit},
		{descriptor: toolAnalyzeQuestion, run: s.toolAnalyzeQuestion},
		{descriptor: toolOrganizeDrafts, run: s.toolOrganizeDrafts},
		{descriptor: toolFillExplanations, run: s.toolFillExplanations},
		{descriptor: toolGenerateQuestions, run: s.toolGenerateQuestions},
		{descriptor: toolGenerateVariants, run: s.toolGenerateVariants},
		{descriptor: toolComposePaper, run: s.toolComposePaper},
		{descriptor: toolListAttachments, run: s.toolListAttachments},
		{descriptor: toolSearchCloud, run: s.toolSearchCloud},
		{descriptor: toolCreateQuestions, run: s.toolCreateQuestions},
		{descriptor: toolUpdateDraft, run: s.toolUpdateDraft},
		{descriptor: toolListPapers, run: s.toolListPapers},
		{descriptor: toolGetPaper, run: s.toolGetPaper},
		{descriptor: toolPaperAudit, run: s.toolPaperAudit},
		{descriptor: toolUpdatePaperDraft, run: s.toolUpdatePaperDraft},
		{descriptor: toolBankStats, run: s.toolBankStats},
		{descriptor: toolFindDuplicates, run: s.toolFindDuplicates},
		{descriptor: toolReviewQueue, run: s.toolReviewQueue},
		{descriptor: toolSearchSimilar, run: s.toolSearchSimilar},
	}
	registry := make(map[string]registeredTool, len(tools))
	for _, item := range tools {
		if item.descriptor.Category == "" {
			item.descriptor.Category = toolCategory(item.descriptor.Name)
		}
		registry[item.descriptor.Name] = item
	}
	return registry
}

func toolCategory(name string) string {
	switch name {
	case "list_papers", "get_paper", "paper_audit", "update_paper_draft", "compose_paper":
		return "paper"
	case "bank_stats", "bank_audit", "find_duplicates", "review_queue", "search_similar_questions":
		return "analytics"
	case "list_attachments", "search_cloud":
		return "document"
	default:
		return "question"
	}
}

// ToolCatalog 返回稳定排序的工具目录。
func (s *Service) ToolCatalog(principal auth.Principal) []ToolDescriptor {
	registry := s.toolRegistry()
	items := make([]ToolDescriptor, 0, len(registry))
	for _, item := range registry {
		items = append(items, item.descriptor)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// RunTool 是唯一的工具执行入口，负责参数校验、权限与审计。
func (s *Service) RunTool(ctx context.Context, principal auth.Principal, toolContext ToolContext, name string, args map[string]any) (ToolResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return ToolResult{}, err
	}
	registry := s.toolRegistry()
	tool, ok := registry[name]
	if !ok {
		return ToolResult{}, validationError("tool", "不支持的工具")
	}
	if args == nil {
		args = map[string]any{}
	}
	if err := validateToolArguments(name, args); err != nil {
		return ToolResult{}, err
	}
	if principal.Role == auth.RoleTeacher {
		if subject := stringArgument(args, "subject"); subject != "" && subject != principal.Subject {
			return ToolResult{}, validationError("subject", "教师只能操作本学科内容")
		}
	}
	toolContext.Principal = principal
	result, err := tool.run(ctx, toolContext, args)
	if err != nil {
		return ToolResult{}, err
	}
	if result.Data == nil {
		result.Data = map[string]any{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	if tool.descriptor.Writes && s.repo != nil {
		targetType := "question"
		targetID := ""
		if len(result.QuestionIDs) > 0 {
			targetID = result.QuestionIDs[0]
		}
		if paperID, ok := result.Data["paper_id"].(string); ok {
			targetType = "paper"
			targetID = paperID
		}
		_ = s.repo.CreateAudit(ctx, principal, AuditLog{
			Action:     "agent_tool_" + name,
			TargetType: targetType,
			TargetID:   targetID,
			RunID:      toolContext.RunID,
			Detail: map[string]any{
				"tool":         name,
				"summary":      result.Summary,
				"question_id":  result.QuestionIDs,
				"arguments":    args,
				"conversation": toolContext.ConversationID,
			},
		})
	}
	return result, nil
}

func (s *Service) toolSearchQuestions(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	limit := intArgument(args, "limit", 20, 1, 50)
	filter := question.QuestionFilter{
		Subject:        stringArgument(args, "subject"),
		Grade:          stringArgument(args, "grade"),
		Status:         stringArgument(args, "status"),
		Category:       stringArgument(args, "category"),
		KnowledgePoint: stringArgument(args, "knowledge_point"),
		Page:           1,
		PageSize:       50,
	}
	if toolContext.Principal.Role == auth.RoleTeacher {
		filter.Subject = toolContext.Principal.Subject
	}
	result, err := s.questions.List(ctx, toolContext.Principal, filter)
	if err != nil {
		return ToolResult{}, err
	}
	keyword := strings.ToLower(strings.TrimSpace(stringArgument(args, "keyword")))
	items := make([]map[string]any, 0, limit)
	for _, item := range result.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(plainText(item.Content)), keyword) {
			continue
		}
		items = append(items, map[string]any{
			"id":               item.ID.String(),
			"content_preview":  truncate(plainText(item.Content), 120),
			"subject":          item.Subject,
			"grade":            item.Grade,
			"question_type":    item.QuestionType,
			"difficulty":       item.Difficulty,
			"status":           item.Status,
			"category":         item.Category,
			"knowledge_points": item.KnowledgePoints,
		})
		if len(items) >= limit {
			break
		}
	}
	return ToolResult{
		Summary: fmt.Sprintf("找到 %d 道题目（共匹配 %d 道）", len(items), result.Total),
		Data:    map[string]any{"items": items, "total": result.Total},
	}, nil
}

func (s *Service) toolGetQuestion(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	id, err := uuidArgument(args, "question_id")
	if err != nil {
		return ToolResult{}, err
	}
	item, err := s.questions.Get(ctx, toolContext.Principal, id)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Summary:     fmt.Sprintf("已读取题目《%s》", truncate(plainText(item.Content), 24)),
		QuestionIDs: []string{item.ID.String()},
		Data: map[string]any{
			"question": toSnapshot(item),
		},
	}, nil
}

func (s *Service) toolBankAudit(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	limit := intArgument(args, "limit", 500, 1, 1000)
	audit, err := s.AuditBank(ctx, toolContext.Principal, limit)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Summary: fmt.Sprintf("体检 %d 道题：重复题 %d 组，缺解析 %d 道，缺知识点 %d 道",
			audit.Total, audit.DuplicateCount, audit.MissingExplanation, audit.MissingKnowledgePoints),
		Data: map[string]any{"audit": audit},
	}, nil
}

func (s *Service) toolAnalyzeQuestion(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	id, err := uuidArgument(args, "question_id")
	if err != nil {
		return ToolResult{}, err
	}
	apply := boolArgument(args, "apply", true)
	result, err := s.analyzeQuestion(ctx, toolContext.Principal, id, apply, toolContext.RunID, stringArgument(args, "model"))
	if err != nil {
		return ToolResult{}, err
	}
	summary := "已完成题目分析；题目非草稿，未自动修改"
	if result.Applied {
		summary = "已完成分析并写回草稿"
	}
	return ToolResult{
		Summary:     summary,
		QuestionIDs: []string{id.String()},
		Data:        map[string]any{"analysis": result.Analysis, "applied": result.Applied},
		Warnings:    result.Analysis.Warnings,
	}, nil
}

func (s *Service) toolOrganizeDrafts(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	params := agentParamsFromArgs(args)
	step, ids, warnings, err := s.agentTag(ctx, toolContext.Principal, params, toolContext.RunID)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Summary: step.Summary, QuestionIDs: ids, Warnings: warnings, Data: map[string]any{"step": step, "question_ids": ids}}, nil
}

func (s *Service) toolFillExplanations(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	params := agentParamsFromArgs(args)
	step, ids, warnings, err := s.agentFill(ctx, toolContext.Principal, params, toolContext.RunID)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Summary: step.Summary, QuestionIDs: ids, Warnings: warnings, Data: map[string]any{"step": step, "question_ids": ids}}, nil
}

func (s *Service) toolGenerateQuestions(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	params := agentParamsFromArgs(args)
	step, result, warnings, err := s.agentGenerate(ctx, toolContext.Principal, params, toolContext.RunID)
	if err != nil {
		return ToolResult{}, err
	}
	data := map[string]any{"step": step}
	if result != nil {
		data["items"] = result.Items
		data["question_ids"] = result.QuestionIDs
	}
	return ToolResult{Summary: step.Summary, QuestionIDs: step.QuestionIDs, Warnings: warnings, Data: data}, nil
}

func (s *Service) toolGenerateVariants(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	params := agentParamsFromArgs(args)
	step, result, warnings, err := s.agentVariants(ctx, toolContext.Principal, params, toolContext.RunID)
	if err != nil {
		return ToolResult{}, err
	}
	data := map[string]any{"step": step}
	if result != nil {
		data["items"] = result.Items
		data["question_ids"] = result.QuestionIDs
	}
	return ToolResult{Summary: step.Summary, QuestionIDs: step.QuestionIDs, Warnings: warnings, Data: data}, nil
}

func (s *Service) toolComposePaper(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	params := agentParamsFromArgs(args)
	step, result, warnings, err := s.agentComposePaper(ctx, toolContext.Principal, params, toolContext.RunID)
	if err != nil {
		return ToolResult{}, err
	}
	ids := []string{}
	if value, ok := result["question_ids"].([]string); ok {
		ids = value
	}
	return ToolResult{Summary: step.Summary, QuestionIDs: ids, Warnings: warnings, Data: result}, nil
}

func (s *Service) toolListAttachments(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	attachments, err := s.loadAttachments(ctx, toolContext.Principal, toolContext.AttachmentIDs)
	if err != nil {
		return ToolResult{}, err
	}
	items := make([]map[string]any, 0, len(attachments))
	for _, item := range attachments {
		entry := map[string]any{
			"id": item.ID, "name": item.Name, "kind": item.Kind,
			"status": item.Status, "size_bytes": item.SizeBytes,
		}
		if item.Kind == AttachmentKindImage {
			entry["mode"] = "vision"
			entry["note"] = "图片已作为视觉输入随本轮消息发送，可直接阅读。"
		} else {
			entry["mode"] = "file"
			entry["note"] = "原文件已随本轮消息直接发送给模型，可直接阅读其内容。"
		}
		items = append(items, entry)
	}
	return ToolResult{
		Summary: fmt.Sprintf("本轮共有 %d 个附件", len(attachments)),
		Data:    map[string]any{"items": items},
	}, nil
}

// toolSearchCloud 检索学校云盘中已向量化的资料，只读取文本片段，不写入任何数据。
func (s *Service) toolSearchCloud(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.cloudDrive == nil {
		return ToolResult{}, validationError("query", "云盘向量检索未启用")
	}
	query := strings.TrimSpace(stringArgument(args, "query"))
	if query == "" {
		return ToolResult{}, validationError("query", "请输入检索关键词")
	}
	if len([]rune(query)) > 200 {
		return ToolResult{}, validationError("query", "检索关键词不能超过200个字符")
	}
	limit := intArgument(args, "limit", 8, 1, 20)
	result, err := s.cloudDrive.Search(ctx, toolContext.Principal, query, limit)
	if err != nil {
		if errors.Is(err, drive.ErrIndexing) {
			return ToolResult{}, validationError("query", "云盘向量检索暂不可用")
		}
		if errors.Is(err, drive.ErrValidation) {
			return ToolResult{}, validationError("query", "检索关键词不合法")
		}
		return ToolResult{}, err
	}
	files := make(map[string]bool, len(result.Hits))
	for _, hit := range result.Hits {
		files[hit.ItemID] = true
	}
	summary := fmt.Sprintf("云盘资料中未找到与「%s」相关的已索引内容", result.Query)
	if len(result.Hits) > 0 {
		summary = fmt.Sprintf("云盘资料中检索到 %d 个相关片段，来自 %d 个文件", len(result.Hits), len(files))
	}
	return ToolResult{
		Summary: summary,
		Data: map[string]any{
			"query":          result.Query,
			"provider":       result.Provider,
			"chunks_scanned": result.ChunksScanned,
			"hits":           result.Hits,
		},
	}, nil
}

func (s *Service) toolCreateQuestions(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	subject := strings.TrimSpace(stringArgument(args, "subject"))
	grade := strings.TrimSpace(stringArgument(args, "grade"))
	if toolContext.Principal.Role == auth.RoleTeacher {
		subject = toolContext.Principal.Subject
	}
	if subject == "" || grade == "" {
		return ToolResult{}, validationError("subject", "创建题目需要学科与年级")
	}
	rawItems, ok := args["items"].([]any)
	if !ok || len(rawItems) == 0 {
		return ToolResult{}, validationError("items", "请提供待创建的题目列表")
	}
	if len(rawItems) > 10 {
		return ToolResult{}, validationError("items", "单次最多创建 10 道题目")
	}
	created := make([]string, 0, len(rawItems))
	warnings := []string{}
	for index, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		category := stringArgument(item, "category")
		knowledgePoints := stringSliceArgument(item, "knowledge_points")
		input := question.Input{
			Subject:         subject,
			Grade:           grade,
			QuestionType:    stringArgument(item, "question_type"),
			Difficulty:      intArgument(item, "difficulty", 3, 1, 5),
			Tags:            dropDuplicateMetadataTags(uniqueStrings(stringSliceArgument(item, "tags")), category, knowledgePoints),
			Category:        category,
			KnowledgePoints: knowledgePoints,
			Content:         stringArgument(item, "content"),
			Options:         objectOptions(item["options"]),
			Answer:          stringArgument(item, "answer"),
			Explanation:     stringArgument(item, "explanation"),
			AIGenerated:     true,
			AIMeta: map[string]any{
				"provider": "agent_chat", "review_required": true,
				"imported_at": time.Now().UTC().Format(time.RFC3339),
			},
		}
		createdQuestion, createErr := s.questions.Create(ctx, toolContext.Principal, input)
		if createErr != nil {
			warnings = append(warnings, fmt.Sprintf("第 %d 题创建失败：%s", index+1, userFacingQuestionError(createErr)))
			continue
		}
		created = append(created, createdQuestion.ID.String())
	}
	return ToolResult{
		Summary:     fmt.Sprintf("已创建 %d 道草稿题目", len(created)),
		QuestionIDs: created,
		Warnings:    warnings,
		Data:        map[string]any{"question_ids": created},
	}, nil
}

func (s *Service) toolUpdateDraft(ctx context.Context, toolContext ToolContext, args map[string]any) (ToolResult, error) {
	if s.questions == nil {
		return ToolResult{}, fmt.Errorf("题库服务不可用")
	}
	id, err := uuidArgument(args, "question_id")
	if err != nil {
		return ToolResult{}, err
	}
	patch := question.AIApplyPatch{
		Provider:       "agent_chat",
		Model:          stringArgument(args, "model"),
		Confidence:     1,
		Rationale:      "用户通过 AI 对话要求修改",
		ReviewRequired: true,
	}
	if value, ok := args["category"].(string); ok && strings.TrimSpace(value) != "" {
		patch.Category = &value
	}
	if value, ok := args["knowledge_points"]; ok {
		values := stringSliceArgument(map[string]any{"value": value}, "value")
		patch.KnowledgePoints = &values
	}
	if value, ok := args["tags"]; ok {
		values := stringSliceArgument(map[string]any{"value": value}, "value")
		patch.Tags = &values
	}
	if _, ok := args["difficulty"]; ok {
		parsed := intArgument(args, "difficulty", 3, 1, 5)
		patch.Difficulty = &parsed
	}
	if value, ok := args["answer"].(string); ok && strings.TrimSpace(value) != "" {
		patch.Answer = &value
	}
	if value, ok := args["explanation"].(string); ok && strings.TrimSpace(value) != "" {
		patch.Explanation = &value
	}
	updated, err := s.questions.ApplyAI(ctx, toolContext.Principal, id, patch)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Summary:     fmt.Sprintf("已更新草稿《%s》", truncate(plainText(updated.Content), 24)),
		QuestionIDs: []string{id.String()},
		Data:        map[string]any{"question": toSnapshot(updated)},
	}, nil
}

func (s *Service) loadAttachments(ctx context.Context, principal auth.Principal, ids []string) ([]Attachment, error) {
	if len(ids) > MaxAttachments {
		return nil, validationError("attachment_ids", "每轮最多使用 5 个附件")
	}
	items := make([]Attachment, 0, len(ids))
	var firstErr error
	for _, raw := range ids {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			if firstErr == nil {
				firstErr = validationError("attachment_ids", "附件ID格式不正确")
			}
			continue
		}
		item, err := s.repo.GetAttachment(ctx, principal, id)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if item.Status == AttachmentStatusFailed {
			if firstErr == nil {
				firstErr = validationError("attachment_ids", fmt.Sprintf("附件《%s》解析失败：%s", item.Name, item.Error))
			}
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return uniqueAttachments(items), nil
}

// resolveAttachments 在模型传入的附件 ID 非法或不存在时，回退到本轮实际附件，避免整次工具调用失败。
func (s *Service) resolveAttachments(ctx context.Context, principal auth.Principal, requested, fallback []string) ([]Attachment, error) {
	if len(requested) == 0 {
		requested = fallback
	}
	items, err := s.loadAttachments(ctx, principal, requested)
	if len(items) > 0 {
		return items, nil
	}
	if len(fallback) > 0 && !sameAttachmentIDs(requested, fallback) {
		if fallbackItems, _ := s.loadAttachments(ctx, principal, fallback); len(fallbackItems) > 0 {
			return fallbackItems, nil
		}
	}
	return items, err
}

func sameAttachmentIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(left[index]) != strings.TrimSpace(right[index]) {
			return false
		}
	}
	return true
}

func agentParamsFromArgs(args map[string]any) AgentParams {
	return AgentParams{
		Subject:        stringArgument(args, "subject"),
		Grade:          stringArgument(args, "grade"),
		Category:       stringArgument(args, "category"),
		KnowledgePoint: stringArgument(args, "knowledge_point"),
		Tag:            stringArgument(args, "tag"),
		QuestionID:     stringArgument(args, "question_id"),
		Count:          intArgument(args, "count", 0, 0, DefaultMaxGenerate),
		Model:          stringArgument(args, "model"),
	}
}

func objectOptions(raw any) []question.Option {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]question.Option, 0, len(items))
	for _, item := range items {
		value, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key := strings.ToUpper(stringArgument(value, "key"))
		content := stringArgument(value, "content")
		if key == "" || content == "" {
			continue
		}
		result = append(result, question.Option{Key: key, Content: content})
	}
	return result
}

func stringArgument(args map[string]any, key string) string {
	value, ok := args[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func intArgument(args map[string]any, key string, fallback, minimum, maximum int) int {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	parsed := 0
	switch typed := value.(type) {
	case float64:
		parsed = int(typed)
	case int:
		parsed = typed
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return fallback
		}
		parsed = int(number)
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return fallback
		}
		parsed = number
	default:
		return fallback
	}
	if parsed < minimum {
		return minimum
	}
	if parsed > maximum {
		return maximum
	}
	return parsed
}

func boolArgument(args map[string]any, key string, fallback bool) bool {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	typed, ok := value.(bool)
	if !ok {
		return fallback
	}
	return typed
}

func uuidArgument(args map[string]any, key string) (uuid.UUID, error) {
	raw := stringArgument(args, key)
	if raw == "" {
		return uuid.Nil, validationError(key, "缺少必填参数 "+key)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, validationError(key, key+" 格式不正确")
	}
	return id, nil
}

func stringSliceArgument(args map[string]any, key string) []string {
	value, ok := args[key]
	if !ok {
		return nil
	}
	if direct, ok := value.([]string); ok {
		result := make([]string, 0, len(direct))
		for _, item := range direct {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	raw, ok := value.([]any)
	if !ok {
		if single, ok := value.(string); ok && strings.TrimSpace(single) != "" {
			parts := strings.Split(single, ",")
			result := make([]string, 0, len(parts))
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					result = append(result, trimmed)
				}
			}
			return result
		}
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}

func validateToolArguments(name string, args map[string]any) error {
	allowed := map[string]map[string]bool{
		"search_questions":         {"keyword": true, "subject": true, "grade": true, "status": true, "category": true, "knowledge_point": true, "limit": true},
		"get_question":             {"question_id": true},
		"bank_audit":               {"limit": true},
		"analyze_question":         {"question_id": true, "apply": true, "model": true},
		"organize_drafts":          {"subject": true, "grade": true, "category": true, "knowledge_point": true, "tag": true, "question_id": true, "count": true, "model": true},
		"fill_explanations":        {"subject": true, "grade": true, "category": true, "knowledge_point": true, "tag": true, "question_id": true, "count": true, "model": true},
		"generate_questions":       {"subject": true, "grade": true, "category": true, "knowledge_point": true, "tag": true, "count": true, "model": true},
		"generate_variants":        {"question_id": true, "count": true, "model": true},
		"compose_paper":            {"subject": true, "grade": true, "category": true, "knowledge_point": true, "tag": true, "question_id": true, "count": true, "model": true},
		"list_attachments":         {},
		"search_cloud":             {"query": true, "limit": true},
		"create_questions":         {"subject": true, "grade": true, "items": true},
		"update_draft":             {"question_id": true, "category": true, "knowledge_points": true, "tags": true, "difficulty": true, "answer": true, "explanation": true, "model": true},
		"list_papers":              {"subject": true, "grade": true, "keyword": true, "limit": true},
		"get_paper":                {"paper_id": true},
		"paper_audit":              {"paper_id": true},
		"update_paper_draft":       {"paper_id": true, "title": true, "duration_minutes": true, "questions": true},
		"bank_stats":               {"subject": true, "grade": true, "limit": true},
		"find_duplicates":          {"limit": true},
		"review_queue":             {"subject": true, "grade": true, "limit": true},
		"search_similar_questions": {"question_id": true, "keyword": true, "subject": true, "grade": true, "limit": true},
	}
	fields, ok := allowed[name]
	if !ok {
		return validationError("tool", "不支持的工具")
	}
	for key, value := range args {
		if !fields[key] {
			return validationError(key, "不接受的工具参数")
		}
		switch value.(type) {
		case map[string]any, []any, []string, string, float64, bool, nil:
		default:
			return validationError(key, "工具参数类型不支持")
		}
	}
	switch name {
	case "get_question", "analyze_question", "update_draft":
		if _, err := uuidArgument(args, "question_id"); err != nil {
			return err
		}
	case "get_paper", "paper_audit", "update_paper_draft":
		if _, err := uuidArgument(args, "paper_id"); err != nil {
			return err
		}
	case "search_similar_questions":
		if raw := stringArgument(args, "question_id"); raw != "" {
			if _, err := uuidArgument(args, "question_id"); err != nil {
				return err
			}
		}
	case "generate_variants":
		if _, err := uuidArgument(args, "question_id"); err != nil {
			return err
		}
	case "search_cloud":
		query := strings.TrimSpace(stringArgument(args, "query"))
		if query == "" {
			return validationError("query", "请输入检索关键词")
		}
		if len([]rune(query)) > 200 {
			return validationError("query", "检索关键词不能超过200个字符")
		}
	}
	return nil
}
