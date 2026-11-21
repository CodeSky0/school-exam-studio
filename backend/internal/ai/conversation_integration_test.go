package ai

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	entquestion "school-exam-studio/backend/ent/question"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestConversationFlowWithDocumentImport(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = client.AIAuditLog.Delete().Exec(ctx)
		_, _ = client.AIAttachment.Delete().Exec(ctx)
		_, _ = client.AIMessage.Delete().Exec(ctx)
		_, _ = client.AIConversation.Delete().Exec(ctx)
		_, _ = client.AIRun.Delete().Exec(ctx)
		_, _ = client.Question.Delete().Where(entquestion.SchoolIDEQ(platformtest.SchoolA)).Exec(ctx)
	})
	service := NewService(Options{
		QuestionService: question.NewService(question.NewRepository(client)),
		Repository:      NewRepository(client),
		UploadDir:       t.TempDir(),
		SettingsSecret:  "integration-settings-secret",
		LLMTimeout:      time.Second,
	})
	teacher := platformtest.TeacherPrincipal(client)
	admin := platformtest.AdminPrincipal(client)
	otherTeacher := auth.Principal{
		UserID: platformtest.OtherTeacherA, SchoolID: platformtest.SchoolA,
		Role: auth.RoleTeacher, Subject: "语文", SessionVersion: 1,
	}

	conversation, err := service.CreateConversation(ctx, teacher, "文档导入测试")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	content := "1. 计算 1+1\n答案：2\n\n2. 下列正确的是（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n答案：B\n"
	attachment, err := service.UploadAttachment(ctx, teacher, conversation.ID, "exam.txt", "text/plain", []byte(content))
	if err != nil {
		t.Fatalf("UploadAttachment: %v", err)
	}
	if attachment.Status != AttachmentStatusParsed {
		t.Fatalf("attachment status = %s error = %s", attachment.Status, attachment.Error)
	}
	if attachment.TextChars == 0 {
		t.Fatal("expected extracted text")
	}

	result, err := service.SendMessage(ctx, teacher, mustUUID(t, conversation.ID), SendMessageInput{
		Content:       "把这份文件里的题导入八年级数学题库",
		AttachmentIDs: []string{attachment.ID},
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if result.AssistantMessage.Status != MessageStatusSucceeded {
		t.Fatalf("assistant status = %s error = %s", result.AssistantMessage.Status, result.AssistantMessage.Error)
	}
	if len(result.AssistantMessage.ToolCalls) != 2 {
		t.Fatalf("tool calls = %#v", result.AssistantMessage.ToolCalls)
	}
	if result.AssistantMessage.ToolCalls[0].Name != "extract_questions" || result.AssistantMessage.ToolCalls[1].Name != "import_document_questions" {
		t.Fatalf("tool call names = %s, %s", result.AssistantMessage.ToolCalls[0].Name, result.AssistantMessage.ToolCalls[1].Name)
	}
	if result.AssistantMessage.ToolCalls[1].Status != ToolStatusSucceeded {
		t.Fatalf("import failed: %s", result.AssistantMessage.ToolCalls[1].Error)
	}
	if len(result.AssistantMessage.ToolCalls[1].Result) == 0 {
		t.Fatal("expected tool result payload")
	}
	ids := messageQuestionIDsForTest(result.AssistantMessage.ToolCalls[1])
	if len(ids) != 2 {
		t.Fatalf("imported IDs = %#v", ids)
	}
	questions, err := client.Question.Query().Where(entquestion.SchoolIDEQ(platformtest.SchoolA)).All(ctx)
	if err != nil {
		t.Fatalf("query questions: %v", err)
	}
	if len(questions) != 2 {
		t.Fatalf("questions = %d", len(questions))
	}
	for _, item := range questions {
		if item.Status != question.StatusDraft || item.Subject != "数学" || item.Grade != "八年级" || !item.AiGenerated {
			t.Fatalf("unexpected question: status=%s subject=%s grade=%s ai=%v", item.Status, item.Subject, item.Grade, item.AiGenerated)
		}
	}

	loadedConversation, messages, err := service.GetConversation(ctx, teacher, mustUUID(t, conversation.ID))
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if loadedConversation.MessageCount != 2 || len(messages) != 2 {
		t.Fatalf("conversation count=%d messages=%d", loadedConversation.MessageCount, len(messages))
	}
	if messages[1].Role != MessageRoleAssistant || len(messages[1].ToolCalls) != 2 {
		t.Fatalf("assistant message = %#v", messages[1])
	}

	if _, _, err := service.GetConversation(ctx, otherTeacher, mustUUID(t, conversation.ID)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other teacher read = %v", err)
	}
	if _, _, err := service.GetConversation(ctx, admin, mustUUID(t, conversation.ID)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("admin read should be forbidden, got %v", err)
	}
	adminList, err := service.ListConversations(ctx, admin, 30)
	if err != nil {
		t.Fatalf("ListConversations admin: %v", err)
	}
	for _, item := range adminList {
		if item.ID == conversation.ID {
			t.Fatal("admin must not see other users' conversations")
		}
	}
	if _, err := service.GetAttachment(ctx, otherTeacher, mustUUID(t, attachment.ID)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other teacher attachment read = %v", err)
	}
	if err := service.DeleteConversation(ctx, teacher, mustUUID(t, conversation.ID)); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	if _, _, err := service.GetConversation(ctx, teacher, mustUUID(t, conversation.ID)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted conversation read = %v", err)
	}
}

func TestConversationMessageWindowReturnsLatestMessages(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = client.AIAttachment.Delete().Exec(ctx)
		_, _ = client.AIMessage.Delete().Exec(ctx)
		_, _ = client.AIConversation.Delete().Exec(ctx)
		_, _ = client.AIRun.Delete().Exec(ctx)
	})
	teacher := platformtest.TeacherPrincipal(client)
	repo := NewRepository(client)
	conversation, err := repo.CreateConversation(ctx, teacher, Conversation{Title: "消息窗口测试"})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	conversationID := mustUUID(t, conversation.ID)
	for index := 0; index < 5; index++ {
		_, err := repo.AppendMessage(ctx, teacher, ChatMessage{
			ConversationID: conversation.ID,
			Role:           MessageRoleUser,
			Content:        fmt.Sprintf("第 %d 条", index),
			Status:         MessageStatusSucceeded,
			Provider:       "user",
		})
		if err != nil {
			t.Fatalf("AppendMessage %d: %v", index, err)
		}
		// 保证每条消息的 created_at 有可区分的先后顺序。
		time.Sleep(2 * time.Millisecond)
	}
	messages, err := repo.ListMessages(ctx, teacher, conversationID, 2)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	if messages[0].Content != "第 3 条" || messages[1].Content != "第 4 条" {
		t.Fatalf("latest window = %q, %q", messages[0].Content, messages[1].Content)
	}
}

func TestFirstTurnAttachmentIsBoundToConversation(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = client.AIAuditLog.Delete().Exec(ctx)
		_, _ = client.AIAttachment.Delete().Exec(ctx)
		_, _ = client.AIMessage.Delete().Exec(ctx)
		_, _ = client.AIConversation.Delete().Exec(ctx)
		_, _ = client.AIRun.Delete().Exec(ctx)
		_, _ = client.Question.Delete().Where(entquestion.SchoolIDEQ(platformtest.SchoolA)).Exec(ctx)
	})
	service := NewService(Options{
		QuestionService: question.NewService(question.NewRepository(client)),
		Repository:      NewRepository(client),
		UploadDir:       t.TempDir(),
		SettingsSecret:  "integration-settings-secret",
		LLMTimeout:      time.Second,
	})
	teacher := platformtest.TeacherPrincipal(client)
	conversation, err := service.CreateConversation(ctx, teacher, "首轮附件测试")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	// 模拟前端在还没有会话时先上传附件。
	attachment, err := service.UploadAttachment(ctx, teacher, "", "notes.txt", "text/plain", []byte("函数与方程复习提纲"))
	if err != nil {
		t.Fatalf("UploadAttachment: %v", err)
	}
	if attachment.ConversationID != "" {
		t.Fatalf("attachment conversation = %q, want empty", attachment.ConversationID)
	}
	if _, err := service.SendMessage(ctx, teacher, mustUUID(t, conversation.ID), SendMessageInput{
		Content:       "你好",
		AttachmentIDs: []string{attachment.ID},
	}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	loaded, err := service.GetAttachment(ctx, teacher, mustUUID(t, attachment.ID))
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}
	if loaded.ConversationID != conversation.ID {
		t.Fatalf("attachment conversation = %q, want %q", loaded.ConversationID, conversation.ID)
	}
	if err := service.DeleteConversation(ctx, teacher, mustUUID(t, conversation.ID)); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	if _, err := service.GetAttachment(ctx, teacher, mustUUID(t, attachment.ID)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("attachment after delete = %v, want not found", err)
	}
}

func messageQuestionIDsForTest(call ToolCall) []string {
	ids := make([]string, 0)
	switch values := call.Result["question_ids"].(type) {
	case []string:
		ids = append(ids, values...)
	case []any:
		for _, item := range values {
			if value, ok := item.(string); ok {
				ids = append(ids, value)
			}
		}
	}
	data, _ := call.Result["data"].(map[string]any)
	if data == nil {
		return ids
	}
	switch values := data["question_ids"].(type) {
	case []string:
		ids = append(ids, values...)
	case []any:
		for _, item := range values {
			if value, ok := item.(string); ok {
				ids = append(ids, value)
			}
		}
	}
	seen := make(map[string]bool, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	return unique
}

func mustUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	parsed, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", value, err)
	}
	return parsed
}
