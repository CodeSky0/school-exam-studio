package ai

import (
	"testing"

	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/internal/auth"
)

func TestToolCallMapRoundTrip(t *testing.T) {
	calls := []ToolCall{{
		ID: "1", Name: "bank_audit", Label: "题库体检", Status: ToolStatusSucceeded,
		Summary: "检查 10 道题", Arguments: map[string]any{"limit": 10},
		Result: map[string]any{"total": 10},
	}}
	roundTrip := mapsToToolCalls(toolCallsToMaps(calls))
	if len(roundTrip) != 1 || roundTrip[0].Name != "bank_audit" || roundTrip[0].Summary != "检查 10 道题" {
		t.Fatalf("roundTrip = %#v", roundTrip)
	}
}

func TestAttachmentRefPreviewIsBounded(t *testing.T) {
	attachment := Attachment{ExtractedText: "第一行\n第二行 附加说明"}
	ref := attachment.Ref()
	if len([]rune(ref.TextPreview)) > 181 {
		t.Fatalf("preview too long: %d", len([]rune(ref.TextPreview)))
	}
	if ref.TextPreview == "" {
		t.Fatal("preview should not be empty")
	}
}

func TestConversationPermissions(t *testing.T) {
	schoolID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	teacher := auth.Principal{Role: auth.RoleTeacher, UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), SchoolID: schoolID}
	otherTeacher := auth.Principal{Role: auth.RoleTeacher, UserID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), SchoolID: schoolID}
	admin := auth.Principal{Role: auth.RoleAdmin, UserID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), SchoolID: schoolID}
	conversation := &ent.AIConversation{SchoolID: teacher.SchoolID, CreatorID: teacher.UserID}
	if !canReadConversation(teacher, conversation) || !canWriteConversation(teacher, conversation) {
		t.Fatal("teacher should read/write own conversation")
	}
	if canReadConversation(otherTeacher, conversation) || canWriteConversation(otherTeacher, conversation) {
		t.Fatal("other teacher must not access conversation")
	}
	if !canReadConversation(admin, conversation) || canWriteConversation(admin, conversation) {
		t.Fatal("admin can read but not write another user's conversation")
	}
}
