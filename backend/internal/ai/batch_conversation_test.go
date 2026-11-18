package ai

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeConversationIDsDeduplicates(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	ids, err := normalizeConversationIDs([]uuid.UUID{first, first, second})
	if err != nil {
		t.Fatalf("normalizeConversationIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != first || ids[1] != second {
		t.Fatalf("ids = %#v", ids)
	}
	if _, err := normalizeConversationIDs(nil); err == nil {
		t.Fatal("empty ids should be rejected")
	}
}

func TestBatchDeleteConversationsRejectsEmptySelection(t *testing.T) {
	service := &Service{}
	if _, err := service.BatchDeleteConversations(context.Background(), testPrincipal(), nil); err == nil {
		t.Fatal("empty selection should be rejected")
	}
}

func TestConversationBatchReason(t *testing.T) {
	if got := conversationBatchReason(ErrForbidden); got != "只能删除自己创建的会话" {
		t.Fatalf("unexpected reason: %s", got)
	}
	if got := conversationBatchReason(ErrNotFound); got != "会话不存在或已被删除" {
		t.Fatalf("unexpected reason: %s", got)
	}
}
