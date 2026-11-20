package drive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	// MaxFileBytes 限制单个云盘文件大小。全局请求体上限需大于该值。
	MaxFileBytes      = 50 << 20
	maxCloudNameRunes = 120
)

type fileType struct {
	kind string
	mime string
}

// 云盘只负责保存原文件，因此比 AI 附件支持更多 Office 格式。
var allowedExtensions = map[string]fileType{
	".pdf":  {kind: "pdf", mime: "application/pdf"},
	".doc":  {kind: "doc", mime: "application/msword"},
	".docx": {kind: "docx", mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
	".ppt":  {kind: "ppt", mime: "application/vnd.ms-powerpoint"},
	".pptx": {kind: "pptx", mime: "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	".xls":  {kind: "xls", mime: "application/vnd.ms-excel"},
	".xlsx": {kind: "xlsx", mime: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	".txt":  {kind: "text", mime: "text/plain"},
	".md":   {kind: "text", mime: "text/markdown"},
	".csv":  {kind: "text", mime: "text/csv"},
	".png":  {kind: "image", mime: "image/png"},
	".jpg":  {kind: "image", mime: "image/jpeg"},
	".jpeg": {kind: "image", mime: "image/jpeg"},
	".webp": {kind: "image", mime: "image/webp"},
	".gif":  {kind: "image", mime: "image/gif"},
}

// SupportedExtensions 返回云盘允许上传的扩展名，供测试与前端约定使用。
func SupportedExtensions() []string {
	result := make([]string, 0, len(allowedExtensions))
	for ext := range allowedExtensions {
		result = append(result, ext)
	}
	return result
}

type FileStore struct {
	root string
}

func NewFileStore(root string) (*FileStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "./data/cloud-drive"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("无法解析云盘目录：%w", err)
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("无法创建云盘目录：%w", err)
	}
	return &FileStore{root: absolute}, nil
}

func (s *FileStore) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *FileStore) Save(schoolID, userID uuid.UUID, name string, data []byte) (StoredFile, error) {
	if s == nil {
		return StoredFile{}, fmt.Errorf("云盘存储未初始化")
	}
	if len(data) == 0 {
		return StoredFile{}, validationError("file", "文件内容为空")
	}
	if len(data) > MaxFileBytes {
		return StoredFile{}, validationError("file", "文件超过 50 MiB 限制")
	}
	cleanName, kind, mime, ext, err := classifyName(name)
	if err != nil {
		return StoredFile{}, err
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	relative := filepath.Join(schoolID.String(), userID.String(), hash[:2], uuid.NewString()+ext)
	absolute := filepath.Join(s.root, relative)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o750); err != nil {
		return StoredFile{}, fmt.Errorf("无法创建云盘目录：%w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(absolute), ".upload-*")
	if err != nil {
		return StoredFile{}, fmt.Errorf("无法写入云盘文件：%w", err)
	}
	tempName := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法写入云盘文件：%w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法写入云盘文件：%w", err)
	}
	if err := os.Chmod(tempName, 0o640); err != nil {
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法设置云盘文件权限：%w", err)
	}
	if err := os.Rename(tempName, absolute); err != nil {
		_ = os.Remove(tempName)
		return StoredFile{}, fmt.Errorf("无法保存云盘文件：%w", err)
	}
	return StoredFile{
		Name:        cleanName,
		MIME:        mime,
		Kind:        kind,
		SizeBytes:   int64(len(data)),
		StoragePath: filepath.ToSlash(relative),
		SHA256:      hash,
	}, nil
}

func (s *FileStore) Open(storagePath string) (*os.File, error) {
	if s == nil {
		return nil, fmt.Errorf("云盘存储未初始化")
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

func (s *FileStore) Remove(storagePath string) error {
	if s == nil || strings.TrimSpace(storagePath) == "" {
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

func (s *FileStore) resolve(storagePath string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(storagePath))
	if cleaned == "." || filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("云盘路径不合法")
	}
	absolute := filepath.Join(s.root, cleaned)
	if !strings.HasPrefix(absolute, s.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("云盘路径不合法")
	}
	return absolute, nil
}

func classifyName(name string) (string, string, string, string, error) {
	cleanName := sanitizeName(name)
	ext := strings.ToLower(filepath.Ext(cleanName))
	info, ok := allowedExtensions[ext]
	if !ok {
		return "", "", "", "", validationError("file", "仅支持 PDF、Word、PPT、Excel、TXT/Markdown/CSV 与常见图片格式")
	}
	return cleanName, info.kind, info.mime, ext, nil
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.Map(func(value rune) rune {
		if unicode.IsControl(value) || value == '/' || value == '\\' {
			return -1
		}
		return value
	}, name)
	if name == "" || name == "." || name == ".." {
		return ""
	}
	if utf8.RuneCountInString(name) > maxCloudNameRunes {
		runes := []rune(name)
		ext := filepath.Ext(name)
		base := runes[:maxCloudNameRunes-len([]rune(ext))]
		name = string(base) + ext
	}
	return name
}
