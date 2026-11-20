package drive

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

func TestFileStoreSavesOfficeFile(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	schoolID, userID := uuid.New(), uuid.New()
	data := bytes.Repeat([]byte("ppt-data"), 32)
	saved, err := store.Save(schoolID, userID, "../../期末复习.PPTX", data)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "期末复习.PPTX" || saved.Kind != "pptx" || saved.MIME == "" {
		t.Fatalf("saved = %#v", saved)
	}
	if strings.Contains(saved.StoragePath, "..") {
		t.Fatalf("path not sanitized: %q", saved.StoragePath)
	}
	file, err := store.Open(saved.StoragePath)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || !bytes.Equal(content, data) {
		t.Fatalf("content mismatch: %v", err)
	}
	if err := store.Remove(saved.StoragePath); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(saved.StoragePath); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestFileStoreRejectsUnsupportedAndOversized(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(uuid.New(), uuid.New(), "run.exe", []byte("MZ")); err == nil {
		t.Fatal("expected unsupported extension rejection")
	}
	oversized := make([]byte, MaxFileBytes+1)
	if _, err := store.Save(uuid.New(), uuid.New(), "big.pdf", oversized); err == nil {
		t.Fatal("expected size rejection")
	}
	if _, err := store.Open("../etc/passwd"); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestNormalizeFolderName(t *testing.T) {
	if name, err := normalizeFolderName("  高一资料  "); err != nil || name != "高一资料" {
		t.Fatalf("name = %q, err = %v", name, err)
	}
	for _, invalid := range []string{"", "  ", ".", "..", "a/b", `a\b`, strings.Repeat("字", 121)} {
		if _, err := normalizeFolderName(invalid); err == nil {
			t.Fatalf("expected %q to be rejected", invalid)
		}
	}
}

func TestValidatePrincipalAllowsTeacherAndAdmin(t *testing.T) {
	base := auth.Principal{UserID: uuid.New(), SchoolID: uuid.New()}
	for _, role := range []string{auth.RoleTeacher, auth.RoleAdmin} {
		principal := base
		principal.Role = role
		if err := validatePrincipal(principal); err != nil {
			t.Fatalf("role %q rejected: %v", role, err)
		}
	}
	principal := base
	principal.Role = "student"
	if err := validatePrincipal(principal); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
