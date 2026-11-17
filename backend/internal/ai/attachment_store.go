package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/document"
)

const (
	MaxImageBytes     = 10 << 20
	MaxDocBytes       = 20 << 20
	MaxAttachments    = 5
	maxAttachmentName = 120
)

type StoredFile struct {
	Name        string
	MIME        string
	Kind        string
	SizeBytes   int64
	StoragePath string
	SHA256      string
}

type AttachmentStore struct {
	root     string
	maxImage int64
	maxDoc   int64
}

func NewAttachmentStore(root string) (*AttachmentStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "./data/ai-uploads"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("无法解析附件目录：%w", err)
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("无法创建附件目录：%w", err)
	}
	return &AttachmentStore{root: absolute, maxImage: MaxImageBytes, maxDoc: MaxDocBytes}, nil
}

func (s *AttachmentStore) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *AttachmentStore) Save(schoolID, userID uuid.UUID, name, mime string, data []byte) (StoredFile, error) {
	if s == nil {
		return StoredFile{}, fmt.Errorf("附件存储未初始化")
	}
	if len(data) == 0 {
		return StoredFile{}, fmt.Errorf("附件内容为空")
	}
	kind, normalizedMIME, ext, limit, err := classifyAttachment(name, mime, data)
	if err != nil {
		return StoredFile{}, err
	}
	if kind == AttachmentKindImage && s.maxImage > 0 && s.maxImage < limit {
		limit = s.maxImage
	}
	if kind != AttachmentKindImage && s.maxDoc > 0 && s.maxDoc < limit {
		limit = s.maxDoc
	}
	if int64(len(data)) > limit {
		return StoredFile{}, fmt.Errorf("附件超过大小限制（最大 %d MiB）", limit>>20)
	}
	cleanName := sanitizeAttachmentName(name, ext)
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	relative := filepath.Join(schoolID.String(), userID.String(), hash[:2], uuid.NewString()+ext)
	absolute := filepath.Join(s.root, relative)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o750); err != nil {
		return StoredFile{}, fmt.Errorf("无法创建附件目录：%w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(absolute), ".upload-*")
	if err != nil {
		return StoredFile{}, fmt.Errorf("无法写入附件：%w", err)
	}
	tempName := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法写入附件：%w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法写入附件：%w", err)
	}
	if err := os.Chmod(tempName, 0o640); err != nil {
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法设置附件权限：%w", err)
	}
	if err := os.Rename(tempName, absolute); err != nil {
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法保存附件：%w", err)
	}
	return StoredFile{
		Name:        cleanName,
		MIME:        normalizedMIME,
		Kind:        kind,
		SizeBytes:   int64(len(data)),
		StoragePath: filepath.ToSlash(relative),
		SHA256:      hash,
	}, nil
}

func (s *AttachmentStore) Open(storagePath string) (io.ReadCloser, error) {
	if s == nil {
		return nil, fmt.Errorf("附件存储未初始化")
	}
	absolute, err := s.resolve(storagePath)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return file, nil
}

func (s *AttachmentStore) Remove(storagePath string) error {
	if s == nil {
		return nil
	}
	absolute, err := s.resolve(storagePath)
	if err != nil {
		return err
	}
	if err := os.Remove(absolute); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *AttachmentStore) resolve(storagePath string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(storagePath))
	if cleaned == "." || filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("附件路径不合法")
	}
	absolute := filepath.Join(s.root, cleaned)
	if !strings.HasPrefix(absolute, s.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("附件路径不合法")
	}
	return absolute, nil
}

func classifyAttachment(name, mime string, data []byte) (string, string, string, int64, error) {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	lowerMIME := strings.ToLower(strings.TrimSpace(mime))
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return AttachmentKindImage, "image/png", ".png", MaxImageBytes, nil
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return AttachmentKindImage, "image/jpeg", ".jpg", MaxImageBytes, nil
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return AttachmentKindImage, "image/gif", ".gif", MaxImageBytes, nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return AttachmentKindImage, "image/webp", ".webp", MaxImageBytes, nil
	}
	kind, err := document.DetectKind(name, mime, data)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("不支持的文件类型：仅支持图片、PDF、DOCX、TXT、Markdown、CSV")
	}
	ext := filepath.Ext(lowerName)
	switch kind {
	case "pdf":
		return AttachmentKindPDF, "application/pdf", ".pdf", MaxDocBytes, nil
	case "docx":
		return AttachmentKindDocx, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx", MaxDocBytes, nil
	case "text":
		switch ext {
		case ".md":
			return AttachmentKindText, "text/markdown", ".md", MaxDocBytes, nil
		case ".csv":
			return AttachmentKindText, "text/csv", ".csv", MaxDocBytes, nil
		}
		if lowerMIME == "text/markdown" {
			return AttachmentKindText, "text/markdown", ".md", MaxDocBytes, nil
		}
		if lowerMIME == "text/csv" {
			return AttachmentKindText, "text/csv", ".csv", MaxDocBytes, nil
		}
		return AttachmentKindText, "text/plain", ".txt", MaxDocBytes, nil
	default:
		return "", "", "", 0, fmt.Errorf("不支持的文件类型")
	}
}

func sanitizeAttachmentName(name, ext string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.Map(func(value rune) rune {
		if unicode.IsControl(value) || value == '/' || value == '\\' {
			return -1
		}
		return value
	}, name)
	if name == "" || name == "." {
		name = "attachment" + ext
	}
	if utf8.RuneCountInString(name) > maxAttachmentName {
		runes := []rune(name)
		suffix := filepath.Ext(name)
		base := string(runes[:maxAttachmentName-len([]rune(suffix))])
		name = base + suffix
	}
	if filepath.Ext(name) == "" && ext != "" {
		name += ext
	}
	return name
}
