package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/document"
)

const (
	maxMessageRunes    = 8000
	maxLLMHistory      = 12
	maxLLMToolRounds   = 6
	maxLLMToolCalls    = 12
	maxToolResultChars = 6000
)

func (s *Service) CreateConversation(ctx context.Context, principal auth.Principal, title string) (Conversation, error) {
	return s.repo.CreateConversation(ctx, principal, Conversation{Title: title})
}

func (s *Service) ListConversations(ctx context.Context, principal auth.Principal, limit int) ([]Conversation, error) {
	return s.repo.ListConversations(ctx, principal, limit)
}

func (s *Service) GetConversation(ctx context.Context, principal auth.Principal, id uuid.UUID) (Conversation, []ChatMessage, error) {
	conversation, err := s.repo.GetConversation(ctx, principal, id)
	if err != nil {
		return Conversation{}, nil, err
	}
	messages, err := s.repo.ListMessages(ctx, principal, id, 200)
	if err != nil {
		return Conversation{}, nil, err
	}
	return conversation, messages, nil
}

func (s *Service) UpdateConversation(ctx context.Context, principal auth.Principal, id uuid.UUID, title string, pinned bool) (Conversation, error) {
	return s.repo.UpdateConversation(ctx, principal, id, title, pinned)
}

func (s *Service) DeleteConversation(ctx context.Context, principal auth.Principal, id uuid.UUID) error {
	return s.repo.DeleteConversation(ctx, principal, id)
}

func (s *Service) ListMessages(ctx context.Context, principal auth.Principal, conversationID uuid.UUID, limit int) ([]ChatMessage, error) {
	return s.repo.ListMessages(ctx, principal, conversationID, limit)
}

// UploadAttachment 保存文件、抽取文本，并把解析结果写入附件记录。
func (s *Service) UploadAttachment(ctx context.Context, principal auth.Principal, conversationID, name, mime string, data []byte) (Attachment, error) {
	if err := validatePrincipal(principal); err != nil {
		return Attachment{}, err
	}
	if s.attachments == nil {
		return Attachment{}, fmt.Errorf("附件存储不可用")
	}
	conversationUUID := uuid.Nil
	if strings.TrimSpace(conversationID) != "" {
		parsed, err := uuid.Parse(conversationID)
		if err != nil {
			return Attachment{}, validationError("conversation_id", "会话ID格式不正确")
		}
		conversationUUID = parsed
		conversation, err := s.repo.GetConversation(ctx, principal, parsed)
		if err != nil {
			return Attachment{}, err
		}
		if conversation.CreatorID != principal.UserID.String() {
			return Attachment{}, ErrForbidden
		}
	}
	stored, err := s.attachments.Save(principal.SchoolID, principal.UserID, name, mime, data)
	if err != nil {
		return Attachment{}, validationError("file", err.Error())
	}
	attachment := Attachment{
		Name:        stored.Name,
		MIME:        stored.MIME,
		Kind:        stored.Kind,
		SizeBytes:   stored.SizeBytes,
		StoragePath: stored.StoragePath,
		SHA256:      stored.SHA256,
		Status:      AttachmentStatusUploaded,
	}
	if conversationUUID != uuid.Nil {
		attachment.ConversationID = conversationUUID.String()
	}
	created, err := s.repo.CreateAttachment(ctx, principal, attachment)
	if err != nil {
		_ = s.attachments.Remove(stored.StoragePath)
		return Attachment{}, err
	}
	return s.parseAttachment(ctx, principal, created)
}

func (s *Service) GetAttachment(ctx context.Context, principal auth.Principal, id uuid.UUID) (Attachment, error) {
	return s.repo.GetAttachment(ctx, principal, id)
}

func (s *Service) OpenAttachment(ctx context.Context, principal auth.Principal, id uuid.UUID) (Attachment, io.ReadCloser, error) {
	attachment, err := s.repo.GetAttachment(ctx, principal, id)
	if err != nil {
		return Attachment{}, nil, err
	}
	if s.attachments == nil {
		return Attachment{}, nil, ErrNotFound
	}
	file, err := s.attachments.Open(attachment.StoragePath)
	if err != nil {
		return Attachment{}, nil, err
	}
	return attachment, file, nil
}

func (s *Service) parseAttachment(ctx context.Context, principal auth.Principal, attachment Attachment) (Attachment, error) {
	if s.attachments == nil {
		return attachment, nil
	}
	file, err := s.attachments.Open(attachment.StoragePath)
	if err != nil {
		return s.failAttachment(ctx, principal, attachment, "附件文件已丢失")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, MaxDocBytes+1))
	_ = file.Close()
	if readErr != nil {
		return s.failAttachment(ctx, principal, attachment, "附件读取失败")
	}
	var text string
	switch attachment.Kind {
	case AttachmentKindImage:
		if s.ocr == nil {
			return s.failAttachment(ctx, principal, attachment, "OCR 服务未配置，暂时无法解析图片")
		}
		text, err = s.ocr.Recognize(ctx, data, attachment.Name, attachment.MIME)
		if err != nil {
			return s.failAttachment(ctx, principal, attachment, "图片 OCR 失败："+err.Error())
		}
	case AttachmentKindPDF, AttachmentKindDocx, AttachmentKindText:
		result, extractErr := document.Extract(attachment.Name, attachment.MIME, data)
		if extractErr != nil {
			return s.failAttachment(ctx, principal, attachment, "文档解析失败："+extractErr.Error())
		}
		text = result.Text
	default:
		return s.failAttachment(ctx, principal, attachment, "不支持的附件类型")
	}
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > document.MaxTextChars {
		text = string(runes[:document.MaxTextChars])
	}
	attachment.ExtractedText = text
	attachment.TextChars = utf8.RuneCountInString(text)
	attachment.Status = AttachmentStatusParsed
	attachment.Error = ""
	if text == "" {
		return s.failAttachment(ctx, principal, attachment, "未从附件中提取到文本")
	}
	attachmentID, parseErr := uuid.Parse(attachment.ID)
	if parseErr != nil {
		return attachment, parseErr
	}
	updated, err := s.repo.UpdateAttachment(ctx, principal, attachmentID, attachment)
	if err != nil {
		return attachment, err
	}
	return updated, nil
}

func (s *Service) failAttachment(ctx context.Context, principal auth.Principal, attachment Attachment, message string) (Attachment, error) {
	attachment.Status = AttachmentStatusFailed
	attachment.Error = message
	attachmentID, parseErr := uuid.Parse(attachment.ID)
	if parseErr != nil {
		return attachment, parseErr
	}
	updated, err := s.repo.UpdateAttachment(ctx, principal, attachmentID, attachment)
	if err != nil {
		return attachment, err
	}
	return updated, nil
}

func (s *Service) SendMessage(ctx context.Context, principal auth.Principal, conversationID uuid.UUID, input SendMessageInput) (SendMessageResult, error) {
	return s.sendMessage(ctx, principal, conversationID, input, nil)
}

// SendMessageStream 在完整回复落库的同时，把用户消息、工具调用与最终消息实时推送给调用方。
func (s *Service) SendMessageStream(ctx context.Context, principal auth.Principal, conversationID uuid.UUID, input SendMessageInput, sink ChatEventSink) (SendMessageResult, error) {
	return s.sendMessage(ctx, principal, conversationID, input, sink)
}

func (s *Service) sendMessage(ctx context.Context, principal auth.Principal, conversationID uuid.UUID, input SendMessageInput, sink ChatEventSink) (SendMessageResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return SendMessageResult{}, err
	}
	content := strings.TrimSpace(input.Content)
	if utf8.RuneCountInString(content) > maxMessageRunes {
		return SendMessageResult{}, validationError("content", "单条消息不能超过 8000 字")
	}
	if len(input.AttachmentIDs) > MaxAttachments {
		return SendMessageResult{}, validationError("attachment_ids", "每轮最多使用 5 个附件")
	}
	conversation, err := s.repo.GetConversation(ctx, principal, conversationID)
	if err != nil {
		return SendMessageResult{}, err
	}
	if conversation.CreatorID != principal.UserID.String() {
		return SendMessageResult{}, ErrForbidden
	}
	attachments := make([]Attachment, 0, len(input.AttachmentIDs))
	for _, raw := range input.AttachmentIDs {
		id, parseErr := uuid.Parse(strings.TrimSpace(raw))
		if parseErr != nil {
			return SendMessageResult{}, validationError("attachment_ids", "附件ID格式不正确")
		}
		attachment, getErr := s.repo.GetAttachment(ctx, principal, id)
		if getErr != nil {
			return SendMessageResult{}, getErr
		}
		if attachment.ConversationID != "" && attachment.ConversationID != conversation.ID {
			return SendMessageResult{}, validationError("attachment_ids", "附件不属于当前会话")
		}
		if attachment.Status == AttachmentStatusUploaded {
			attachment, getErr = s.parseAttachment(ctx, principal, attachment)
			if getErr != nil {
				return SendMessageResult{}, getErr
			}
		}
		if attachment.Status == AttachmentStatusFailed {
			return SendMessageResult{}, validationError("attachment_ids", fmt.Sprintf("附件《%s》解析失败：%s", attachment.Name, attachment.Error))
		}
		attachments = append(attachments, attachment)
	}
	if content == "" && len(attachments) == 0 {
		return SendMessageResult{}, validationError("content", "消息内容或附件至少需要一项")
	}
	refs := make([]AttachmentRef, 0, len(attachments))
	for _, item := range attachments {
		refs = append(refs, item.Ref())
	}
	history, err := s.repo.ListMessages(ctx, principal, conversationID, 40)
	if err != nil {
		return SendMessageResult{}, err
	}
	userMessage, err := s.repo.AppendMessage(ctx, principal, ChatMessage{
		ConversationID: conversation.ID,
		Role:           MessageRoleUser,
		Content:        content,
		Status:         MessageStatusSucceeded,
		Provider:       "user",
		Attachments:    refs,
	})
	if err != nil {
		return SendMessageResult{}, err
	}
	emitChatEvent(sink, ChatStreamEvent{Type: ChatEventUserMessage, Message: &userMessage})
	if conversation.MessageCount == 0 {
		title := content
		if title == "" && len(refs) > 0 {
			title = refs[0].Name
		}
		title = textPreview(title, 30)
		if title != "" {
			if updated, updateErr := s.repo.UpdateConversation(ctx, principal, conversationID, title, conversation.Pinned); updateErr == nil {
				conversation = updated
			}
		}
	}
	runID := uuid.New().String()
	if _, err := s.repo.CreateRun(ctx, principal, AgentRun{
		ID:       runID,
		Goal:     firstNonEmpty(content, "附件对话"),
		Intent:   IntentChat,
		Status:   MessageStatusSucceeded,
		Provider: ProviderLocal,
		Plan:     []Step{},
		Steps:    []Step{},
		Result:   map[string]any{"conversation_id": conversation.ID, "attachment_ids": input.AttachmentIDs},
	}); err != nil {
		return SendMessageResult{}, err
	}
	reply, toolCalls, provider, model, warnings := s.runChatTurn(ctx, principal, conversation, history, content, attachments, input.Model, runID, sink)
	status := MessageStatusSucceeded
	errorMessage := ""
	if reply == "" {
		status = MessageStatusFailed
		errorMessage = "AI 未返回可用回复"
		reply = "抱歉，本轮没有生成可用回复。你可以换一种说法或稍后重试。"
	}
	if len(warnings) > 0 {
		reply = strings.TrimSpace(reply + "\n\n> " + strings.Join(warnings, "；"))
	}
	assistantMessage, err := s.repo.AppendMessage(ctx, principal, ChatMessage{
		ConversationID: conversation.ID,
		Role:           MessageRoleAssistant,
		Content:        reply,
		Status:         status,
		Provider:       provider,
		Model:          model,
		ToolCalls:      toolCalls,
		RunID:          runID,
		Error:          errorMessage,
	})
	if err != nil {
		return SendMessageResult{}, err
	}
	updated, err := s.repo.GetConversation(ctx, principal, conversationID)
	if err != nil {
		return SendMessageResult{}, err
	}
	return SendMessageResult{
		Conversation:       updated,
		UserMessage:        userMessage,
		AssistantMessage:   assistantMessage,
		AttachmentDelivery: refs,
	}, nil
}

func (s *Service) runChatTurn(ctx context.Context, principal auth.Principal, conversation Conversation, history []ChatMessage, content string, attachments []Attachment, model, runID string, sink ChatEventSink) (string, []ToolCall, string, string, []string) {
	warnings := []string{}
	candidates, warning, err := s.resolveCandidates(ctx, principal, model)
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if err != nil {
		return err.Error(), nil, ProviderLLM, "", warnings
	}
	lastModel := ""
	for _, candidate := range candidates {
		lastModel = candidate.Config.Model
		if !s.breaker().allow(candidate.Key, time.Now()) {
			warnings = append(warnings, candidate.Config.Model+" 暂时熔断，已跳过")
			continue
		}
		reply, calls, llmErr := s.runLLMTurn(ctx, principal, conversation, history, content, attachments, candidate.Config.Model, runID, candidate, sink)
		if llmErr == nil {
			return reply, calls, ProviderLLM, candidate.Config.Model, warnings
		}
		if len(calls) > 0 {
			warnings = append(warnings, "模型调用中断，本轮已执行的工具结果保留如下；请确认后决定是否继续："+llmErr.Error())
			summary := summarizeToolCalls(calls)
			if summary == "" {
				summary = "模型调用中断，本轮未能完成回复。"
			}
			return summary, calls, ProviderFallback, candidate.Config.Model, warnings
		}
		warnings = append(warnings, fmt.Sprintf("%s 调用失败：%v", candidate.Config.Model, llmErr))
		if kind := errorKindOf(llmErr); kind == ErrorKindConfig || kind == ErrorKindFatal {
			break
		}
		s.breaker().failure(candidate.Key, time.Now())
	}
	if len(candidates) > 0 && !s.allowLocalFallback() {
		reason := "模型调用失败"
		if len(warnings) > 0 {
			reason += "：" + strings.Join(warnings, "；")
		} else if err != nil {
			reason += "：" + err.Error()
		}
		return reason, []ToolCall{}, ProviderLLM, lastModel, warnings
	}
	refs := make([]AttachmentRef, 0, len(attachments))
	for _, item := range attachments {
		refs = append(refs, item.Ref())
	}
	plan := planChatTurn(content, refs, history, principal)
	if len(plan.Calls) == 0 {
		return plan.Reply, []ToolCall{}, ProviderLocal, "", warnings
	}
	toolContext := ToolContext{Principal: principal, ConversationID: conversation.ID, RunID: runID}
	for _, ref := range refs {
		toolContext.AttachmentIDs = append(toolContext.AttachmentIDs, ref.ID)
	}
	calls := make([]ToolCall, 0, len(plan.Calls))
	for _, planned := range plan.Calls {
		arguments := withModelArgument(planned.Name, planned.Arguments, model)
		call := s.executeToolCall(ctx, principal, toolContext, planned.Name, arguments, sink)
		calls = append(calls, call)
		if call.Status == ToolStatusFailed {
			break
		}
	}
	reply := plan.Reply
	if reply == "" || len(calls) > 0 {
		reply = summarizeToolCalls(calls)
	}
	return reply, calls, ProviderLocal, "", warnings
}

func (s *Service) runLLMTurn(ctx context.Context, principal auth.Principal, conversation Conversation, history []ChatMessage, content string, attachments []Attachment, model, runID string, candidate modelCandidate, sink ChatEventSink) (string, []ToolCall, error) {
	descriptors := s.ToolCatalog(principal)
	llmTools := make([]LLMTool, 0, len(descriptors))
	for _, item := range descriptors {
		llmTools = append(llmTools, LLMTool{
			Type: "function",
			Function: LLMToolFunction{
				Name: item.Name, Description: item.Description, Parameters: item.Parameters,
			},
		})
	}
	messages := []LLMMessage{{
		Role: "system",
		Content: "你是 School Exam Studio 的题库 AI Agent。你可以通过工具检索、分析、整理、生成题目并组卷；" +
			"通过文档工具把 PDF/Word/图片中的题目导入题库。所有自动写入只作用于草稿，必须提醒用户人工复核；" +
			"已发布题目只读。不要编造题目 ID、文件内容或执行结果。回答使用简体中文，先给结论，再列出关键题目 ID 或文件。" +
			"如果缺少学科、年级、题目 ID 或附件，先向用户澄清，不要猜测。",
	}}
	start := len(history) - maxLLMHistory
	if start < 0 {
		start = 0
	}
	for _, item := range history[start:] {
		if item.Role != MessageRoleUser && item.Role != MessageRoleAssistant {
			continue
		}
		text := textPreview(item.Content, 2000)
		if item.Role == MessageRoleAssistant && len(item.ToolCalls) > 0 {
			text += "\n" + previousTurnToolContext(item.ToolCalls)
		}
		if text == "" {
			continue
		}
		messages = append(messages, LLMMessage{Role: item.Role, Content: text})
	}
	refs := make([]AttachmentRef, 0, len(attachments))
	for _, item := range attachments {
		refs = append(refs, item.Ref())
	}
	userContent := content + previewAttachmentContext(refs, 1500)
	if strings.TrimSpace(userContent) == "" {
		userContent = "请查看本轮附件。"
	}
	messages = append(messages, LLMMessage{Role: "user", Content: userContent})
	toolContext := ToolContext{Principal: principal, ConversationID: conversation.ID, RunID: runID}
	for _, ref := range refs {
		toolContext.AttachmentIDs = append(toolContext.AttachmentIDs, ref.ID)
	}
	calls := make([]ToolCall, 0, maxLLMToolCalls)
	narration := strings.Builder{}
	streamedThisRound := false
	for round := 0; round < maxLLMToolRounds; round++ {
		if round > 0 && streamedThisRound {
			// 中间轮次的说明与最终答复之间留出分段，避免不同轮次文字粘在一起。
			emitChatEvent(sink, ChatStreamEvent{Type: ChatEventAssistantDelta, Content: "\n\n"})
			streamedThisRound = false
		}
		reply, err := s.chatToolsStream(ctx, candidate, messages, llmTools, func(delta LLMDelta) {
			if delta.Content != "" {
				streamedThisRound = true
				emitChatEvent(sink, ChatStreamEvent{Type: ChatEventAssistantDelta, Content: delta.Content})
			}
			if delta.Reasoning != "" {
				emitChatEvent(sink, ChatStreamEvent{Type: ChatEventReasoningDelta, Reasoning: delta.Reasoning})
			}
		})
		if err != nil {
			// 已经向用户输出过 Token 时保留这部分回复，避免整段消失。
			if strings.TrimSpace(reply.Content) != "" {
				return composeAgentReply(narration.String(), reply.Content), calls, nil
			}
			return "", calls, err
		}
		if len(reply.ToolCalls) == 0 {
			if strings.TrimSpace(reply.Content) == "" {
				return composeAgentReply(narration.String(), summarizeToolCalls(calls)), calls, nil
			}
			return composeAgentReply(narration.String(), reply.Content), calls, nil
		}
		if text := strings.TrimSpace(reply.Content); text != "" {
			if narration.Len() > 0 {
				narration.WriteString("\n\n")
			}
			narration.WriteString(text)
		}
		messages = append(messages, LLMMessage{Role: "assistant", Content: reply.Content, ToolCalls: reply.ToolCalls})
		for _, llmCall := range reply.ToolCalls {
			if len(calls) >= maxLLMToolCalls {
				break
			}
			arguments := map[string]any{}
			if strings.TrimSpace(llmCall.Function.Arguments) != "" {
				if err := json.Unmarshal([]byte(llmCall.Function.Arguments), &arguments); err != nil {
					arguments = map[string]any{}
				}
			}
			arguments = withModelArgument(llmCall.Function.Name, arguments, model)
			call := s.executeToolCall(ctx, principal, toolContext, llmCall.Function.Name, arguments, sink)
			calls = append(calls, call)
			messages = append(messages, LLMMessage{
				Role:       "tool",
				ToolCallID: llmCall.ID,
				Name:       llmCall.Function.Name,
				Content:    toolResultContent(call),
			})
		}
		if len(calls) >= maxLLMToolCalls {
			break
		}
	}
	reply := summarizeToolCalls(calls)
	if reply == "" {
		reply = "本轮工具调用达到上限，如需继续请回复“继续”。"
	}
	return composeAgentReply(narration.String(), reply), calls, nil
}

// composeAgentReply 保留中间轮次已经展示给用户的说明，再拼接最终答复。
func composeAgentReply(narration, reply string) string {
	narration = strings.TrimSpace(narration)
	reply = strings.TrimSpace(reply)
	switch {
	case narration == "":
		return reply
	case reply == "":
		return narration
	default:
		return narration + "\n\n" + reply
	}
}

func (s *Service) executeToolCall(ctx context.Context, principal auth.Principal, toolContext ToolContext, name string, arguments map[string]any, sink ChatEventSink) ToolCall {
	started := time.Now()
	call := ToolCall{
		ID:        uuid.NewString(),
		Name:      name,
		Label:     s.toolLabel(name),
		Status:    ToolStatusRunning,
		Arguments: arguments,
		StartedAt: &started,
	}
	emitChatEvent(sink, ChatStreamEvent{Type: ChatEventToolStarted, ToolCall: &call})
	result, err := s.RunTool(ctx, principal, toolContext, name, arguments)
	finished := time.Now()
	call.FinishedAt = &finished
	if err != nil {
		call.Status = ToolStatusFailed
		call.Error = userFacingToolError(err)
		call.Summary = "执行失败：" + call.Error
		emitChatEvent(sink, ChatStreamEvent{Type: ChatEventToolFinished, ToolCall: &call})
		return call
	}
	call.Status = ToolStatusSucceeded
	call.Summary = result.Summary
	call.Result = map[string]any{
		"summary":      result.Summary,
		"data":         result.Data,
		"warnings":     result.Warnings,
		"question_ids": result.QuestionIDs,
	}
	emitChatEvent(sink, ChatStreamEvent{Type: ChatEventToolFinished, ToolCall: &call})
	return call
}

func (s *Service) toolLabel(name string) string {
	tool, ok := s.toolRegistry()[name]
	if !ok {
		return name
	}
	return tool.descriptor.Label
}

func userFacingToolError(err error) string {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.Violation.Message
	}
	if errors.Is(err, ErrForbidden) {
		return "没有操作权限"
	}
	if errors.Is(err, ErrNotFound) {
		return "数据不存在或已被删除"
	}
	return err.Error()
}

func toolResultContent(call ToolCall) string {
	payload := map[string]any{
		"status":  call.Status,
		"summary": call.Summary,
		"error":   call.Error,
	}
	if call.Result != nil {
		payload["result"] = call.Result
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return call.Summary
	}
	return textPreview(string(raw), maxToolResultChars)
}

func withModelArgument(name string, arguments map[string]any, model string) map[string]any {
	if model == "" {
		return arguments
	}
	switch name {
	case "analyze_question", "organize_drafts", "fill_explanations", "generate_questions", "generate_variants", "compose_paper", "update_draft":
		if _, exists := arguments["model"]; !exists {
			arguments["model"] = model
		}
	}
	return arguments
}

func toolNames(calls []ToolCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Label)
	}
	return names
}

// previousTurnToolContext 把上一轮工具结果回填到模型上下文，支持用户说“继续”时接着完成任务。
func previousTurnToolContext(calls []ToolCall) string {
	builder := strings.Builder{}
	builder.WriteString("上一轮工具结果（继续任务时参考，勿重复执行）：")
	for _, call := range calls {
		builder.WriteString("\n- ")
		builder.WriteString(call.Label)
		builder.WriteString("：")
		builder.WriteString(firstNonEmpty(call.Summary, call.Error, call.Status))
		if ids := toolCallQuestionIDs(call); len(ids) > 0 {
			if len(ids) > 5 {
				ids = ids[:5]
			}
			builder.WriteString("；题目ID：")
			builder.WriteString(strings.Join(ids, "、"))
		}
	}
	return truncate(builder.String(), 1500)
}

func toolCallQuestionIDs(call ToolCall) []string {
	value, ok := call.Result["question_ids"]
	if !ok {
		return nil
	}
	switch ids := value.(type) {
	case []string:
		return ids
	case []any:
		result := make([]string, 0, len(ids))
		for _, item := range ids {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}
