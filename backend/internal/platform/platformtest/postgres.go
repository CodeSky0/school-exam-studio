package platformtest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/internal/platform/database"
	"school-exam-studio/backend/internal/platform/migrator"
)

func Open(t *testing.T) *ent.Client {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = startEmbedded(t)
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
	for name, deleteRows := range map[string]func() error{
		"paper_questions": func() error { _, err := client.PaperQuestion.Delete().Exec(ctx); return err },
		"papers":          func() error { _, err := client.Paper.Delete().Exec(ctx); return err },
		"questions":       func() error { _, err := client.Question.Delete().Exec(ctx); return err },
		"users":           func() error { _, err := client.User.Delete().Exec(ctx); return err },
		"schools":         func() error { _, err := client.School.Delete().Exec(ctx); return err },
	} {
		if err := deleteRows(); err != nil {
			client.Close()
			t.Fatalf("clean %s: %v", name, err)
		}
	}
	t.Cleanup(func() { client.Close() })
	return client
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
