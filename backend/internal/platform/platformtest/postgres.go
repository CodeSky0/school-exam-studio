package platformtest

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/lib/pq"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/internal/platform/database"
	"school-exam-studio/backend/internal/platform/migrator"
)

func Open(t *testing.T) *ent.Client {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = startEmbedded(t)
	} else {
		databaseURL = packageDatabaseURL(t, databaseURL)
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || !strings.Contains(strings.ToLower(parsed.Path), "test") {
		t.Fatal("TEST_DATABASE_URL must reference a dedicated test database")
	}
	if err := migrator.Up(databaseURL); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	client, err := database.Open(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	ctx := context.Background()
	for _, table := range []struct {
		name       string
		deleteRows func() error
	}{
		{"ai_audit_logs", func() error { _, err := client.AIAuditLog.Delete().Exec(ctx); return err }},
		{"ai_attachments", func() error { _, err := client.AIAttachment.Delete().Exec(ctx); return err }},
		{"ai_messages", func() error { _, err := client.AIMessage.Delete().Exec(ctx); return err }},
		{"ai_conversations", func() error { _, err := client.AIConversation.Delete().Exec(ctx); return err }},
		{"ai_runs", func() error { _, err := client.AIRun.Delete().Exec(ctx); return err }},
		{"paper_questions", func() error { _, err := client.PaperQuestion.Delete().Exec(ctx); return err }},
		{"papers", func() error { _, err := client.Paper.Delete().Exec(ctx); return err }},
		{"questions", func() error { _, err := client.Question.Delete().Exec(ctx); return err }},
		{"users", func() error { _, err := client.User.Delete().Exec(ctx); return err }},
		{"schools", func() error { _, err := client.School.Delete().Exec(ctx); return err }},
	} {
		if err := table.deleteRows(); err != nil {
			client.Close()
			t.Fatalf("clean %s: %v", table.name, err)
		}
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func packageDatabaseURL(t *testing.T, base string) string {
	t.Helper()
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	baseName := strings.TrimPrefix(parsed.Path, "/")
	if baseName == "" || !strings.Contains(strings.ToLower(baseName), "test") {
		t.Fatal("TEST_DATABASE_URL must reference a dedicated test database")
	}
	safeName := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(t.Name(), "_")
	if len(safeName) > 32 {
		safeName = safeName[:32]
	}
	sum := sha1.Sum([]byte(t.Name()))
	databaseName := fmt.Sprintf("%s_%s_%x", baseName, safeName, sum[:4])
	adminURL := *parsed
	adminURL.Path = "/postgres"
	admin, err := sql.Open("postgres", adminURL.String())
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	defer admin.Close()
	var exists bool
	if err := admin.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, databaseName).Scan(&exists); err != nil {
		t.Fatalf("check test database %s: %v", databaseName, err)
	}
	if !exists {
		if _, err := admin.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(databaseName)); err != nil {
			var recreateErr error
			if queryErr := admin.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, databaseName).Scan(&exists); queryErr != nil {
				recreateErr = queryErr
			} else if !exists {
				recreateErr = err
			}
			if recreateErr != nil {
				t.Fatalf("create test database %s: %v", databaseName, recreateErr)
			}
		}
	}
	parsed.Path = "/" + databaseName
	return parsed.String()
}

func startEmbedded(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate PostgreSQL test port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release PostgreSQL test port: %v", err)
	}
	cachePath := filepath.Join(os.TempDir(), "school-exam-studio-postgres-cache")
	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		t.Fatalf("create PostgreSQL test cache: %v", err)
	}
	var logs bytes.Buffer
	server := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("user").
		Password("password").
		Database("school_db_test").
		Version(embeddedpostgres.V16).
		Port(uint32(port)).
		RuntimePath(t.TempDir()).
		CachePath(cachePath).
		StartTimeout(90 * time.Second).
		Logger(io.Writer(&logs)))
	if err := server.Start(); err != nil {
		t.Fatalf("start embedded PostgreSQL: %v\n%s", err, logs.String())
	}
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Errorf("stop embedded PostgreSQL: %v", err)
		}
	})
	return fmt.Sprintf("postgres://user:password@127.0.0.1:%d/school_db_test?sslmode=disable", port)
}
