package platformtest_test

import (
	"context"
	"testing"

	"school-exam-studio/backend/internal/platform/platformtest"
)

func TestMigrationsRunOnRealPostgreSQL(t *testing.T) {
	client := platformtest.Open(t)
	if _, err := client.School.Query().Count(context.Background()); err != nil {
		t.Fatalf("query migrated schools table: %v", err)
	}
	if _, err := client.PaperQuestion.Query().Count(context.Background()); err != nil {
		t.Fatalf("query migrated paper questions table: %v", err)
	}
}
