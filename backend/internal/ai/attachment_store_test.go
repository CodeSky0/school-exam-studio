package ai

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAttachmentStoreSavesPNG(t *testing.T) {
	root := t.TempDir()
	store, err := NewAttachmentStore(root)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0x01}, 64)...)
	schoolID, userID := uuid.New(), uuid.New()
	saved, err := store.Save(schoolID, userID, "../../lesson.png", "application/octet-stream", data)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Kind != AttachmentKindImage || saved.MIME != "image/png" {
		t.Fatalf("saved = %#v", saved)
	}
	if strings.Contains(saved.Name, "..") || strings.Contains(saved.Name, "/") {
		t.Fatalf("name not sanitized: %q", saved.Name)
	}
	if !strings.Contains(saved.StoragePath, schoolID.String()) || !strings.Contains(saved.StoragePath, userID.String()) {
		t.Fatalf("path = %q", saved.StoragePath)
	}
	file, err := store.Open(saved.StoragePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil || len(content) != len(data) {
		t.Fatalf("saved file mismatch: %v", err)
	}
	if err := store.Remove(saved.StoragePath); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(saved.StoragePath); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestAttachmentStoreRejectsExecutable(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Save(uuid.New(), uuid.New(), "virus.exe", "application/octet-stream", []byte("MZ\x90\x00"))
	if err == nil {
		t.Fatal("expected rejection")
	}
}

func TestAttachmentStoreRejectsOversizedImage(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.maxImage = 8
	data := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0x00}, 16)...)
	if _, err := store.Save(uuid.New(), uuid.New(), "big.png", "image/png", data); err == nil {
		t.Fatal("expected size rejection")
	}
}

func TestAttachmentStoreRejectsTraversal(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open("../etc/passwd"); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestAttachmentStoreRootCreated(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "uploads")
	if _, err := NewAttachmentStore(root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatalf("root not created: %v", err)
	}
}

func TestSanitizeAttachmentName(t *testing.T) {
	name := sanitizeAttachmentName("../../etc/试卷.docx", ".docx")
	if name != "试卷.docx" {
		t.Fatalf("name = %q", name)
	}
	if got := sanitizeAttachmentName("", ".pdf"); got != "attachment.pdf" {
		t.Fatalf("empty name = %q", got)
	}
}
