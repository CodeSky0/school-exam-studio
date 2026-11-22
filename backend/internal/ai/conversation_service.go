package ai

import (
	"context"
	"encoding/base64"
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
	"school-exam-studio/backend/internal/drive"
)

const (
	maxMessageRunes    = 8000
	maxLLMHistory      = 12
	maxLLMToolRounds   = 6
	maxLLMToolCalls    = 12
	maxToolResultChars = 6000

	// 视觉输入上限：单张图片沿用 10 MiB。
	maxVisionImageBytes = 10 << 20

	// 一轮对话里文本与页面图片的合计上限。
	maxAttachmentTotalBytes = 32 << 20
	// 纯文本与抽取出的文档文本直接内联到消息中。
	maxInlineTextRunes = 100000
	// PDF 文本层不可用时，最多把多少页内嵌图片交给视觉模型。
	maxPDFImagePages = 20
	// 少于该字数的 PDF 文本视为不可用，回退到页面图片。
	minPDFTextRunes = 20
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
	if err := s.repo.DeleteConversation(ctx, principal, id); err != nil {
		return err
	}
	s.purgeConversationAttachments(ctx, principal, id)
	return nil
}

// purgeConversationAttachments 删除会话时一并回收附件文件与记录，避免长期占用磁盘与数据库。
func (s *Service) purgeConversationAttachments(ctx context.Context, principal auth.Principal, conversationID uuid.UUID) {
	items, err := s.repo.ListConversationAttachments(ctx, principal.SchoolID, conversationID)
	if err != nil {
		return
	}
	for _, item := range items {
		attachmentID, parseErr := uuid.Parse(item.ID)
		if parseErr != nil {
			continue
		}
		if s.attachments != nil && item.StoragePath != "" {
			_ = s.attachments.Remove(item.StoragePath)
		}
		_ = s.repo.DeleteAttachmentRecord(ctx, principal.SchoolID, attachmentID)
	}
}

// BatchDeleteConversations 批量软删除会话；只允许删除自己创建的会话，单条失败不影响其余条目。
func (s *Service) BatchDeleteConversations(ctx context.Context, principal auth.Principal, ids []uuid.UUID) (BatchConversationResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return BatchConversationResult{}, err
	}
	unique, err := normalizeConversationIDs(ids)
	if err != nil {
		return BatchConversationResult{}, err
	}
	result := BatchConversationResult{Total: len(unique), Deleted: []string{}, Failed: []BatchConversationErr{}}
	for _, id := range unique {
		if err := s.DeleteConversation(ctx, principal, id); err != nil {
			result.Failed = append(result.Failed, BatchConversationErr{ID: id.String(), Error: conversationBatchReason(err)})
			continue
		}
		result.Deleted = append(result.Deleted, id.String())
	}
	return result, nil
}

func normalizeConversationIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	unique := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, validationError("ids", "会话ID格式不正确")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, validationError("ids", "请至少选择一个会话")
	}
	if len(unique) > MaxBatchConversations {
		return nil, validationError("ids", fmt.Sprintf("单次最多删除 %d 个会话", MaxBatchConversations))
	}
	return unique, nil
}

func conversationBatchReason(err error) string {
	switch {
	case errors.Is(err, ErrForbidden):
		return "只能删除自己创建的会话"
	case errors.Is(err, ErrNotFound):
		return "会话不存在或已被删除"
	default:
		return err.Error()
	}
}

func (s *Service) ListMessages(ctx context.Context, principal auth.Principal, conversationID uuid.UUID, limit int) ([]ChatMessage, error) {
	return s.repo.ListMessages(ctx, principal, conversationID, limit)
}

// UploadAttachment 保存文件、抽取文本，并把解析结果写入附件记录。
func (s *Service) UploadAttachment(ctx context.Context, principal auth.Principal, conversationID, name, mime string, data []byte) (Attachment, error) {
	return s.uploadAttachment(ctx, principal, conversationID, name, mime, data, true)
}

func (s *Service) uploadAttachment(ctx context.Context, principal auth.Principal, conversationID, name, mime string, data []byte, syncToCloud bool) (Attachment, error) {
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
	if syncToCloud && s.cloudDrive != nil {
		// 自动归档是附加能力，失败不阻断对话附件上传。
		_ = s.cloudDrive.ImportFromAI(ctx, principal, stored.Name, data)
	}
	return s.parseAttachment(ctx, principal, created)
}

// UploadCloudAttachments 把云盘中的文件复制为当前对话可发送的附件。
func (s *Service) UploadCloudAttachments(ctx context.Context, principal auth.Principal, ids []string) ([]AttachmentRef, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if s.cloudDrive == nil {
		return nil, validationError("cloud_file_ids", "云盘功能未启用")
	}
	if len(ids) == 0 {
		return nil, validationError("cloud_file_ids", "请选择云盘文件")
	}
	if len(ids) > MaxAttachments {
		return nil, validationError("cloud_file_ids", "每轮最多选择 5 个云盘文件")
	}
	refs := make([]AttachmentRef, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, raw := range ids {
		raw = strings.TrimSpace(raw)
		if seen[raw] {
			continue
		}
		seen[raw] = true
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, validationError("cloud_file_ids", "云盘文件ID格式不正确")
		}
		file, err := s.cloudDrive.ReadForAI(ctx, principal, id)
		if err != nil {
			return nil, translateCloudError(err)
		}
		attachment, err := s.uploadAttachment(ctx, principal, "", file.Name, file.MIME, file.Data, false)
		if err != nil {
			return nil, err
		}
		refs = append(refs, attachment.Ref())
	}
	return refs, nil
}

func translateCloudError(err error) error {
	var validation *drive.ValidationError
	if errors.As(err, &validation) {
		return validationError("cloud_file_ids", validation.Message)
	}
	switch {
	case errors.Is(err, drive.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, drive.ErrForbidden):
		return ErrForbidden
	default:
		return err
	}
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
	// 附件不在服务端抽取文本：PDF/Word 保留原始文件，发送消息时直接交给模型阅读。
	attachment.ExtractedText = ""
	attachment.TextChars = 0
	attachment.Status = AttachmentStatusParsed
	attachment.Error = ""
	return s.persistAttachment(ctx, principal, attachment)
}

func (s *Service) persistAttachment(ctx context.Context, principal auth.Principal, attachment Attachment) (Attachment, error) {
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
	// 上传时未选择会话的附件在首次使用时补绑到当前会话，保证删除会话时能回收文件。
	for _, attachment := range attachments {
		if attachment.ConversationID != "" {
			continue
		}
		attachmentID, parseErr := uuid.Parse(attachment.ID)
		if parseErr != nil {
			continue
		}
		_ = s.repo.BindAttachmentToConversation(ctx, principal.SchoolID, attachmentID, conversationID)
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
	reply, toolCalls, timeline, provider, model, warnings := s.runChatTurn(ctx, principal, conversation, history, content, attachments, input.Model, runID, sink)
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
		Timeline:       timeline,
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

func (s *Service) runChatTurn(ctx context.Context, principal auth.Principal, conversation Conversation, history []ChatMessage, content string, attachments []Attachment, model, runID string, sink ChatEventSink) (string, []ToolCall, []TimelineEntry, string, string, []string) {
	warnings := []string{}
	candidates, warning, err := s.resolveCandidates(ctx, principal, model)
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if err != nil {
		return err.Error(), nil, []TimelineEntry{}, ProviderLLM, "", warnings
	}
	lastModel := ""
	for _, candidate := range candidates {
		lastModel = candidate.Config.Model
		if !s.breaker().allow(candidate.Key, time.Now()) {
			warnings = append(warnings, candidate.Config.Model+" 暂时熔断，已跳过")
			continue
		}
		reply, calls, timeline, llmErr := s.runLLMTurn(ctx, principal, conversation, history, content, attachments, candidate.Config.Model, runID, candidate, sink)
		if llmErr == nil {
			return reply, calls, timeline, ProviderLLM, candidate.Config.Model, warnings
		}
		if len(calls) > 0 {
			warnings = append(warnings, "模型调用中断，本轮已执行的工具结果保留如下；请确认后决定是否继续："+llmErr.Error())
			summary := summarizeToolCalls(calls)
			if summary == "" {
				summary = "模型调用中断，本轮未能完成回复。"
			}
			return summary, calls, timeline, ProviderFallback, candidate.Config.Model, warnings
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
		// reason 已经拼接了全部失败原因，这里不再返回 warnings，避免回复里重复出现同一段信息。
		return reason, []ToolCall{}, []TimelineEntry{}, ProviderLLM, lastModel, nil
	}
	refs := make([]AttachmentRef, 0, len(attachments))
	for _, item := range attachments {
		refs = append(refs, item.Ref())
	}
	plan := planChatTurn(content, refs, history, principal)
	if len(plan.Calls) == 0 {
		return plan.Reply, []ToolCall{}, []TimelineEntry{}, ProviderLocal, "", warnings
	}
	toolContext := ToolContext{Principal: principal, ConversationID: conversation.ID, RunID: runID}
	for _, ref := range refs {
		toolContext.AttachmentIDs = append(toolContext.AttachmentIDs, ref.ID)
	}
	calls := make([]ToolCall, 0, len(plan.Calls))
	timeline := make([]TimelineEntry, 0, len(plan.Calls))
	for _, planned := range plan.Calls {
		arguments := withModelArgument(planned.Name, planned.Arguments, model)
		call := s.executeToolCall(ctx, principal, toolContext, planned.Name, arguments, sink)
		calls = append(calls, call)
		entryTime := time.Now()
		if call.StartedAt != nil {
			entryTime = *call.StartedAt
		}
		timeline = append(timeline, TimelineEntry{Type: TimelineTypeTool, ToolCallID: call.ID, CreatedAt: entryTime})
		if call.Status == ToolStatusFailed {
			break
		}
	}
	reply := plan.Reply
	if reply == "" || len(calls) > 0 {
		reply = summarizeToolCalls(calls)
	}
	return reply, calls, timeline, ProviderLocal, "", warnings
}

// attachmentParts 把本轮附件转成模型可接受的内容块。
// 模型只接受文本和标准图片：PDF/DOCX 先做本地文本抽取，PDF 文本层不可用时再提取内嵌页面图片，
// 不再发送服务端不支持的 file 内容块。
func (s *Service) attachmentParts(attachments []Attachment) ([]LLMContentPart, []string) {
	if s.attachments == nil {
		return nil, nil
	}
	parts := make([]LLMContentPart, 0, len(attachments))
	warnings := make([]string, 0)
	total := 0
	for _, item := range attachments {
		if item.Status != AttachmentStatusParsed {
			warnings = append(warnings, fmt.Sprintf("附件《%s》未上传成功，已跳过", item.Name))
			continue
		}
		limit := int64(maxVisionImageBytes)
		if item.Kind != AttachmentKindImage {
			limit = MaxDocBytes
		}
		if item.SizeBytes > limit {
			if item.Kind == AttachmentKindImage {
				warnings = append(warnings, fmt.Sprintf("图片《%s》超过 %d MiB，已跳过视觉输入", item.Name, limit>>20))
			} else {
				warnings = append(warnings, fmt.Sprintf("文件《%s》超过 %d MiB，已跳过", item.Name, limit>>20))
			}
			continue
		}
		data, ok := s.readAttachment(item, limit)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("附件《%s》读取失败或超过大小限制，已跳过", item.Name))
			continue
		}
		switch item.Kind {
		case AttachmentKindImage:
			if total+len(data) > maxAttachmentTotalBytes {
				warnings = append(warnings, fmt.Sprintf("本轮附件总大小超过 %d MiB，后续附件已跳过", maxAttachmentTotalBytes>>20))
				return parts, warnings
			}
			total += len(data)
			parts = append(parts, imageContentPart(item.MIME, data))
		case AttachmentKindText:
			text, textWarnings := inlineAttachmentText(item.Name, "", 0, string(data))
			warnings = append(warnings, textWarnings...)
			parts = append(parts, LLMContentPart{Type: "text", Text: text})
		case AttachmentKindPDF:
			pdfParts, pdfWarnings, added := s.pdfAttachmentParts(item, data, total)
			warnings = append(warnings, pdfWarnings...)
			total += added
			parts = append(parts, pdfParts...)
		case AttachmentKindDocx:
			result, err := document.Extract(item.Name, item.MIME, data)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("文档《%s》无法读取：%s", item.Name, err.Error()))
				continue
			}
			text, textWarnings := inlineAttachmentText(item.Name, "DOCX", result.Pages, result.Text)
			warnings = append(warnings, result.Warnings...)
			warnings = append(warnings, textWarnings...)
			parts = append(parts, LLMContentPart{Type: "text", Text: text})
		default:
			warnings = append(warnings, fmt.Sprintf("附件《%s》类型不受支持，已跳过", item.Name))
		}
	}
	return parts, warnings
}

func (s *Service) readAttachment(item Attachment, limit int64) ([]byte, bool) {
	file, err := s.attachments.Open(item.StoragePath)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(data) == 0 || int64(len(data)) > limit {
		return nil, false
	}
	return data, true
}

// pdfAttachmentParts 优先发送文本层；文本不足或不可读时，改为发送内嵌页面图片。
func (s *Service) pdfAttachmentParts(item Attachment, data []byte, used int) ([]LLMContentPart, []string, int) {
	warnings := make([]string, 0)
	result, extractErr := document.Extract(item.Name, item.MIME, data)
	if extractErr == nil {
		runes := utf8.RuneCountInString(strings.TrimSpace(result.Text))
		if runes >= minPDFTextRunes && document.LooksReadable(result.Text) {
			text, textWarnings := inlineAttachmentText(item.Name, "PDF", result.Pages, result.Text)
			warnings = append(warnings, result.Warnings...)
			warnings = append(warnings, textWarnings...)
			return []LLMContentPart{{Type: "text", Text: text}}, warnings, len(text)
		}
	}

	images, imageWarnings := document.ExtractPDFImages(data)
	pageParts := make([]LLMContentPart, 0, len(images))
	total := used
	sent := 0
	for _, image := range images {
		if sent >= maxPDFImagePages {
			warnings = append(warnings, fmt.Sprintf("PDF《%s》页面较多，仅发送前 %d 页图片", item.Name, maxPDFImagePages))
			break
		}
		if len(image.Data) > maxVisionImageBytes {
			warnings = append(warnings, fmt.Sprintf("PDF《%s》第 %d 页图片超过 %d MiB，已跳过", item.Name, image.Page, maxVisionImageBytes>>20))
			continue
		}
		if total+len(image.Data) > maxAttachmentTotalBytes {
			warnings = append(warnings, fmt.Sprintf("本轮附件总大小超过 %d MiB，PDF《%s》剩余页面已跳过", maxAttachmentTotalBytes>>20, item.Name))
			break
		}
		total += len(image.Data)
		pageParts = append(pageParts, imageContentPart(image.MIME, image.Data))
		sent++
	}
	if len(pageParts) == 0 {
		// 没有可用页面图片时，短但可读的文本仍然比直接报错更有价值。
		if extractErr == nil && strings.TrimSpace(result.Text) != "" && document.LooksReadable(result.Text) {
			text, textWarnings := inlineAttachmentText(item.Name, "PDF", result.Pages, result.Text)
			warnings = append(warnings, result.Warnings...)
			warnings = append(warnings, textWarnings...)
			return []LLMContentPart{{Type: "text", Text: text}}, warnings, len(text)
		}
		warnings = append(warnings, imageWarnings...)
		reason := "未提取到可读文本"
		if extractErr != nil {
			reason = extractErr.Error()
		}
		warnings = append(warnings, fmt.Sprintf("文档《%s》无法读取（%s），也没有可用的页面图片；请上传页面截图或直接粘贴文字", item.Name, reason))
		return nil, warnings, 0
	}
	intro := fmt.Sprintf("【附件：%s（PDF，已转为 %d 页图片）】请直接阅读这些页面图片。", item.Name, sent)
	parts := make([]LLMContentPart, 0, len(pageParts)+1)
	parts = append(parts, LLMContentPart{Type: "text", Text: intro})
	parts = append(parts, pageParts...)
	return parts, warnings, total - used
}

func inlineAttachmentText(name, kind string, pages int, text string) (string, []string) {
	warnings := make([]string, 0)
	if runes := []rune(text); len(runes) > maxInlineTextRunes {
		text = string(runes[:maxInlineTextRunes])
		warnings = append(warnings, fmt.Sprintf("文档《%s》超过 %d 字，已截断", name, maxInlineTextRunes))
	}
	header := "【附件：" + name
	if kind != "" {
		header += "（" + kind
		if pages > 0 {
			header += fmt.Sprintf("，共 %d 页", pages)
		}
		header += "）"
	}
	header += "】\n"
	return header + text, warnings
}

func imageContentPart(mime string, data []byte) LLMContentPart {
	if mime == "" {
		mime = "image/png"
	}
	return LLMContentPart{
		Type:     "image_url",
		ImageURL: &LLMImageURL{URL: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)},
	}
}

func (s *Service) runLLMTurn(ctx context.Context, principal auth.Principal, conversation Conversation, history []ChatMessage, content string, attachments []Attachment, model, runID string, candidate modelCandidate, sink ChatEventSink) (string, []ToolCall, []TimelineEntry, error) {
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
		Role:    "system",
		Content: chatAgentSystemPrompt,
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
	userContent := content + previewAttachmentContext(refs)
	if strings.TrimSpace(userContent) == "" {
		userContent = "请查看本轮附件。"
	}
	attachmentParts, partWarnings := s.attachmentParts(attachments)
	if len(partWarnings) > 0 {
		userContent += "\n\n附件提示：" + strings.Join(partWarnings, "；")
	}
	userMessage := LLMMessage{Role: "user", Content: userContent}
	if len(attachmentParts) > 0 {
		userMessage.Parts = append([]LLMContentPart{{Type: "text", Text: userContent}}, attachmentParts...)
	}
	messages = append(messages, userMessage)
	toolContext := ToolContext{Principal: principal, ConversationID: conversation.ID, RunID: runID}
	for _, ref := range refs {
		toolContext.AttachmentIDs = append(toolContext.AttachmentIDs, ref.ID)
	}
	calls := make([]ToolCall, 0, maxLLMToolCalls)
	timeline := make([]TimelineEntry, 0, maxLLMToolCalls*2)
	streamedThisRound := false
	for round := 0; round < maxLLMToolRounds; round++ {
		roundStarted := time.Now()
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
				return strings.TrimSpace(reply.Content), calls, timeline, nil
			}
			return "", calls, timeline, err
		}
		if len(reply.ToolCalls) == 0 {
			if reasoning := strings.TrimSpace(reply.Reasoning); reasoning != "" {
				timeline = append(timeline, TimelineEntry{Type: TimelineTypeThought, Content: reasoning, CreatedAt: roundStarted})
			}
			finalReply := strings.TrimSpace(reply.Content)
			if finalReply == "" {
				finalReply = summarizeToolCalls(calls)
			}
			return finalReply, calls, timeline, nil
		}
		// 当轮仍要调用工具，因此本轮的自然语言与推理都属于过程，按时间顺序记入时间线。
		if reasoning := strings.TrimSpace(reply.Reasoning); reasoning != "" {
			timeline = append(timeline, TimelineEntry{Type: TimelineTypeThought, Content: reasoning, CreatedAt: roundStarted})
		}
		if text := strings.TrimSpace(reply.Content); text != "" {
			timeline = append(timeline, TimelineEntry{Type: TimelineTypeThought, Content: text, CreatedAt: roundStarted})
		}
		messages = append(messages, LLMMessage{Role: "assistant", Content: reply.Content, ToolCalls: reply.ToolCalls})
		for _, llmCall := range reply.ToolCalls {
			if len(calls) >= maxLLMToolCalls {
				break
			}
			// 部分模型会在工具名前后带入空白字符，清洗后再匹配注册表，避免整次调用失败。
			toolName := strings.TrimSpace(llmCall.Function.Name)
			arguments := map[string]any{}
			if strings.TrimSpace(llmCall.Function.Arguments) != "" {
				if err := json.Unmarshal([]byte(llmCall.Function.Arguments), &arguments); err != nil {
					arguments = map[string]any{}
				}
			}
			arguments = withModelArgument(toolName, arguments, model)
			call := s.executeToolCall(ctx, principal, toolContext, toolName, arguments, sink)
			calls = append(calls, call)
			entryTime := time.Now()
			if call.StartedAt != nil {
				entryTime = *call.StartedAt
			}
			timeline = append(timeline, TimelineEntry{Type: TimelineTypeTool, ToolCallID: call.ID, CreatedAt: entryTime})
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
	return reply, calls, timeline, nil
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
