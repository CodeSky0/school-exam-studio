package ai

import (
	"context"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/aiattachment"
	"school-exam-studio/backend/ent/aiconversation"
	"school-exam-studio/backend/ent/aimessage"
	"school-exam-studio/backend/internal/auth"
)

func canReadConversation(principal auth.Principal, entity *ent.AIConversation) bool {
	if principal.Role == auth.RoleAdmin {
		return entity.SchoolID == principal.SchoolID
	}
	return entity.CreatorID == principal.UserID
}

func canWriteConversation(principal auth.Principal, entity *ent.AIConversation) bool {
	return entity.SchoolID == principal.SchoolID && entity.CreatorID == principal.UserID
}

func (r *Repository) CreateConversation(ctx context.Context, principal auth.Principal, conversation Conversation) (Conversation, error) {
	if err := validatePrincipal(principal); err != nil {
		return Conversation{}, err
	}
	if r.client == nil {
		return Conversation{}, ErrNotFound
	}
	title := strings.TrimSpace(conversation.Title)
	if title == "" {
		title = "新对话"
	}
	if len([]rune(title)) > 60 {
		title = string([]rune(title)[:60])
	}
	status := conversation.Status
	if status == "" {
		status = ConversationStatusActive
	}
	create := r.client.AIConversation.Create().
		SetSchoolID(principal.SchoolID).
		SetCreatorID(principal.UserID).
		SetTitle(title).
		SetStatus(status).
		SetPinned(conversation.Pinned).
		SetLastMessageAt(time.Now())
	if conversation.ID != "" {
		id, err := uuid.Parse(conversation.ID)
		if err != nil {
			return Conversation{}, validationError("id", "会话ID格式不正确")
		}
		create = create.SetID(id)
	}
	entity, err := create.Save(ctx)
	if err != nil {
		return Conversation{}, err
	}
	return conversationFromEnt(entity), nil
}

func (r *Repository) ListConversations(ctx context.Context, principal auth.Principal, limit int) ([]Conversation, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if r.client == nil {
		return []Conversation{}, nil
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	query := r.client.AIConversation.Query().Where(
		aiconversation.SchoolIDEQ(principal.SchoolID),
		aiconversation.DeletedAtIsNil(),
	)
	if principal.Role != auth.RoleAdmin {
		query = query.Where(aiconversation.CreatorIDEQ(principal.UserID))
	}
	entities, err := query.
		Order(
			aiconversation.ByPinned(entsql.OrderDesc()),
			aiconversation.ByLastMessageAt(entsql.OrderDesc()),
			aiconversation.ByID(entsql.OrderDesc()),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Conversation, 0, len(entities))
	for _, entity := range entities {
		items = append(items, conversationFromEnt(entity))
	}
	return items, nil
}

func (r *Repository) GetConversation(ctx context.Context, principal auth.Principal, id uuid.UUID) (Conversation, error) {
	entity, err := r.getConversationEntity(ctx, principal, id)
	if err != nil {
		return Conversation{}, err
	}
	if !canReadConversation(principal, entity) {
		return Conversation{}, ErrForbidden
	}
	return conversationFromEnt(entity), nil
}

func (r *Repository) UpdateConversation(ctx context.Context, principal auth.Principal, id uuid.UUID, title string, pinned bool) (Conversation, error) {
	entity, err := r.getConversationEntity(ctx, principal, id)
	if err != nil {
		return Conversation{}, err
	}
	if !canWriteConversation(principal, entity) {
		return Conversation{}, ErrForbidden
	}
	update := entity.Update()
	if value := strings.TrimSpace(title); value != "" {
		if len([]rune(value)) > 60 {
			value = string([]rune(value)[:60])
		}
		update = update.SetTitle(value)
	}
	update = update.SetPinned(pinned)
	updated, err := update.Save(ctx)
	if err != nil {
		return Conversation{}, err
	}
	return conversationFromEnt(updated), nil
}

func (r *Repository) DeleteConversation(ctx context.Context, principal auth.Principal, id uuid.UUID) error {
	entity, err := r.getConversationEntity(ctx, principal, id)
	if err != nil {
		return err
	}
	if !canWriteConversation(principal, entity) {
		return ErrForbidden
	}
	return entity.Update().SetDeletedAt(time.Now()).Exec(ctx)
}

func (r *Repository) AppendMessage(ctx context.Context, principal auth.Principal, message ChatMessage) (ChatMessage, error) {
	if err := validatePrincipal(principal); err != nil {
		return ChatMessage{}, err
	}
	if r.client == nil {
		return ChatMessage{}, ErrNotFound
	}
	conversationID, err := uuid.Parse(message.ConversationID)
	if err != nil {
		return ChatMessage{}, validationError("conversation_id", "会话ID格式不正确")
	}
	entity, err := r.getConversationEntity(ctx, principal, conversationID)
	if err != nil {
		return ChatMessage{}, err
	}
	if !canWriteConversation(principal, entity) {
		return ChatMessage{}, ErrForbidden
	}
	now := time.Now()
	if message.ToolCalls == nil {
		message.ToolCalls = []ToolCall{}
	}
	if message.Timeline == nil {
		message.Timeline = []TimelineEntry{}
	}
	if message.Attachments == nil {
		message.Attachments = []AttachmentRef{}
	}
	if message.Status == "" {
		message.Status = MessageStatusSucceeded
	}
	if message.Role == "" {
		message.Role = MessageRoleUser
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return ChatMessage{}, err
	}
	create := tx.AIMessage.Create().
		SetSchoolID(principal.SchoolID).
		SetConversationID(conversationID).
		SetRole(message.Role).
		SetContent(message.Content).
		SetStatus(message.Status).
		SetProvider(message.Provider).
		SetModel(message.Model).
		SetToolCalls(toolCallsToMaps(message.ToolCalls)).
		SetTimeline(timelineToMaps(message.Timeline)).
		SetAttachments(attachmentsToMaps(message.Attachments)).
		SetError(message.Error)
	if message.ID != "" {
		id, parseErr := uuid.Parse(message.ID)
		if parseErr != nil {
			_ = tx.Rollback()
			return ChatMessage{}, validationError("id", "消息ID格式不正确")
		}
		create = create.SetID(id)
	}
	if message.RunID != "" {
		if runID, parseErr := uuid.Parse(message.RunID); parseErr == nil {
			create = create.SetRunID(runID)
		}
	}
	saved, err := create.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return ChatMessage{}, err
	}
	if _, err := tx.AIConversation.UpdateOneID(conversationID).
		AddMessageCount(1).
		SetLastMessageAt(now).
		Save(ctx); err != nil {
		_ = tx.Rollback()
		return ChatMessage{}, err
	}
	if err := tx.Commit(); err != nil {
		return ChatMessage{}, err
	}
	return messageFromEnt(saved), nil
}

func (r *Repository) ListMessages(ctx context.Context, principal auth.Principal, conversationID uuid.UUID, limit int) ([]ChatMessage, error) {
	entity, err := r.getConversationEntity(ctx, principal, conversationID)
	if err != nil {
		return nil, err
	}
	if !canReadConversation(principal, entity) {
		return nil, ErrForbidden
	}
	if r.client == nil {
		return []ChatMessage{}, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	entities, err := r.client.AIMessage.Query().
		Where(
			aimessage.SchoolIDEQ(principal.SchoolID),
			aimessage.ConversationIDEQ(conversationID),
			aimessage.DeletedAtIsNil(),
		).
		Order(
			aimessage.ByCreatedAt(entsql.OrderAsc()),
			aimessage.ByID(entsql.OrderAsc()),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ChatMessage, 0, len(entities))
	for _, item := range entities {
		items = append(items, messageFromEnt(item))
	}
	return items, nil
}

func (r *Repository) CreateAttachment(ctx context.Context, principal auth.Principal, attachment Attachment) (Attachment, error) {
	if err := validatePrincipal(principal); err != nil {
		return Attachment{}, err
	}
	if r.client == nil {
		return Attachment{}, ErrNotFound
	}
	create := r.client.AIAttachment.Create().
		SetSchoolID(principal.SchoolID).
		SetCreatorID(principal.UserID).
		SetName(attachment.Name).
		SetMime(attachment.MIME).
		SetKind(attachment.Kind).
		SetSizeBytes(attachment.SizeBytes).
		SetStoragePath(attachment.StoragePath).
		SetSha256(attachment.SHA256).
		SetExtractedText(attachment.ExtractedText).
		SetTextChars(attachment.TextChars).
		SetStatus(attachment.Status).
		SetError(attachment.Error)
	if attachment.ID != "" {
		id, err := uuid.Parse(attachment.ID)
		if err != nil {
			return Attachment{}, validationError("id", "附件ID格式不正确")
		}
		create = create.SetID(id)
	}
	if attachment.ConversationID != "" {
		conversationID, err := uuid.Parse(attachment.ConversationID)
		if err != nil {
			return Attachment{}, validationError("conversation_id", "会话ID格式不正确")
		}
		create = create.SetConversationID(conversationID)
	}
	entity, err := create.Save(ctx)
	if err != nil {
		return Attachment{}, err
	}
	return attachmentFromEnt(entity), nil
}

func (r *Repository) GetAttachment(ctx context.Context, principal auth.Principal, id uuid.UUID) (Attachment, error) {
	if err := validatePrincipal(principal); err != nil {
		return Attachment{}, err
	}
	if r.client == nil {
		return Attachment{}, ErrNotFound
	}
	entity, err := r.client.AIAttachment.Query().Where(
		aiattachment.SchoolIDEQ(principal.SchoolID),
		aiattachment.IDEQ(id),
		aiattachment.DeletedAtIsNil(),
	).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return Attachment{}, ErrNotFound
		}
		return Attachment{}, err
	}
	if principal.Role != auth.RoleAdmin && entity.CreatorID != principal.UserID {
		return Attachment{}, ErrForbidden
	}
	return attachmentFromEnt(entity), nil
}

func (r *Repository) UpdateAttachment(ctx context.Context, principal auth.Principal, id uuid.UUID, attachment Attachment) (Attachment, error) {
	existing, err := r.GetAttachment(ctx, principal, id)
	if err != nil {
		return Attachment{}, err
	}
	if r.client == nil {
		return Attachment{}, ErrNotFound
	}
	entity, err := r.client.AIAttachment.UpdateOneID(id).
		Where(aiattachment.SchoolIDEQ(principal.SchoolID), aiattachment.DeletedAtIsNil()).
		SetExtractedText(attachment.ExtractedText).
		SetTextChars(attachment.TextChars).
		SetStatus(attachment.Status).
		SetError(attachment.Error).
		Save(ctx)
	if err != nil {
		return Attachment{}, err
	}
	existing.ExtractedText = attachment.ExtractedText
	existing.TextChars = attachment.TextChars
	existing.Status = attachment.Status
	existing.Error = attachment.Error
	return attachmentFromEnt(entity), nil
}

// ListConversationAttachments 返回会话下的附件记录，供删除会话时回收磁盘文件与数据库行。
func (r *Repository) ListConversationAttachments(ctx context.Context, schoolID, conversationID uuid.UUID) ([]Attachment, error) {
	if r.client == nil {
		return []Attachment{}, nil
	}
	entities, err := r.client.AIAttachment.Query().Where(
		aiattachment.SchoolIDEQ(schoolID),
		aiattachment.ConversationIDEQ(conversationID),
		aiattachment.DeletedAtIsNil(),
	).All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Attachment, 0, len(entities))
	for _, entity := range entities {
		items = append(items, attachmentFromEnt(entity))
	}
	return items, nil
}

// DeleteAttachmentRecord 物理删除附件记录，避免删除会话后遗留孤立的文本与元数据。
func (r *Repository) DeleteAttachmentRecord(ctx context.Context, schoolID, id uuid.UUID) error {
	if r.client == nil {
		return nil
	}
	err := r.client.AIAttachment.DeleteOneID(id).Where(aiattachment.SchoolIDEQ(schoolID)).Exec(ctx)
	if ent.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *Repository) getConversationEntity(ctx context.Context, principal auth.Principal, id uuid.UUID) (*ent.AIConversation, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if r.client == nil {
		return nil, ErrNotFound
	}
	entity, err := r.client.AIConversation.Query().Where(
		aiconversation.SchoolIDEQ(principal.SchoolID),
		aiconversation.IDEQ(id),
		aiconversation.DeletedAtIsNil(),
	).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return entity, nil
}

func conversationFromEnt(entity *ent.AIConversation) Conversation {
	return Conversation{
		ID:            entity.ID.String(),
		Title:         entity.Title,
		Status:        entity.Status,
		Pinned:        entity.Pinned,
		CreatorID:     entity.CreatorID.String(),
		MessageCount:  entity.MessageCount,
		LastMessageAt: entity.LastMessageAt,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
}

func messageFromEnt(entity *ent.AIMessage) ChatMessage {
	message := ChatMessage{
		ID:             entity.ID.String(),
		ConversationID: entity.ConversationID.String(),
		Role:           entity.Role,
		Content:        entity.Content,
		Status:         entity.Status,
		Provider:       entity.Provider,
		Model:          entity.Model,
		ToolCalls:      mapsToToolCalls(entity.ToolCalls),
		Timeline:       mapsToTimeline(entity.Timeline),
		Attachments:    mapsToAttachments(entity.Attachments),
		Error:          entity.Error,
		CreatedAt:      entity.CreatedAt,
	}
	if entity.RunID != nil {
		message.RunID = entity.RunID.String()
	}
	return message
}

func attachmentFromEnt(entity *ent.AIAttachment) Attachment {
	attachment := Attachment{
		ID:            entity.ID.String(),
		CreatorID:     entity.CreatorID.String(),
		Name:          entity.Name,
		MIME:          entity.Mime,
		Kind:          entity.Kind,
		SizeBytes:     entity.SizeBytes,
		StoragePath:   entity.StoragePath,
		SHA256:        entity.Sha256,
		ExtractedText: entity.ExtractedText,
		TextChars:     entity.TextChars,
		Status:        entity.Status,
		Error:         entity.Error,
		CreatedAt:     entity.CreatedAt,
	}
	if entity.ConversationID != nil {
		attachment.ConversationID = entity.ConversationID.String()
	}
	return attachment
}

func uniqueAttachments(items []Attachment) []Attachment {
	seen := map[string]bool{}
	result := make([]Attachment, 0, len(items))
	for _, item := range items {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		result = append(result, item)
	}
	return result
}
