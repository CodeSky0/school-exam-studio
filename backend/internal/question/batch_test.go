package question

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

func TestNormalizeBatchIDsDeduplicates(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	ids, err := normalizeBatchIDs([]uuid.UUID{first, first, second})
	if err != nil {
		t.Fatalf("normalizeBatchIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != first || ids[1] != second {
		t.Fatalf("ids = %#v", ids)
	}
	if _, err := normalizeBatchIDs(nil); err == nil {
		t.Fatal("empty ids should be rejected")
	}
	tooMany := make([]uuid.UUID, MaxBatchQuestions+1)
	for index := range tooMany {
		tooMany[index] = uuid.New()
	}
	if _, err := normalizeBatchIDs(tooMany); err == nil {
		t.Fatal("too many ids should be rejected")
	}
}

func TestBatchUpdateStatusRejectsInvalidStatus(t *testing.T) {
	service := NewService(nil)
	_, err := service.BatchUpdateStatus(context.Background(), auth.Principal{
		UserID: uuid.New(), SchoolID: uuid.New(), Role: auth.RoleAdmin,
	}, []uuid.UUID{uuid.New()}, StatusDraft)
	if err == nil {
		t.Fatal("draft status should be rejected for batch updates")
	}
}

func TestBatchFailureReason(t *testing.T) {
	cases := map[error]string{
		ErrForbidden:         "无权操作，或题目不是可操作的草稿",
		ErrInvalidTransition: "当前状态不允许该状态流转",
		ErrNotFound:          "题目不存在或已被删除",
	}
	for source, want := range cases {
		if got := batchFailureReason(source); got != want {
			t.Fatalf("batchFailureReason(%v) = %q, want %q", source, got, want)
		}
	}
}
