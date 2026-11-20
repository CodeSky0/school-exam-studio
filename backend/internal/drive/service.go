package drive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

const (
	maxFolderNameRunes = 120
	maxDeleteDepth     = 5000
	maxBreadcrumbDepth = 32
)

type Service struct {
	repository *Repository
	store      *FileStore
}

func NewService(repository *Repository, store *FileStore) *Service {
	return &Service{repository: repository, store: store}
}

// List 返回某个目录的直接子节点；parentID 为空时表示云盘根目录。
func (s *Service) List(ctx context.Context, principal auth.Principal, parentID string) (ListResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return ListResult{}, err
	}
	if s.repository == nil {
		return ListResult{}, ErrNotFound
	}
	parsed, err := parseOptionalID(parentID, "parent_id")
	if err != nil {
		return ListResult{}, err
	}
	if parsed != nil {
		parent, err := s.repository.Get(ctx, principal.SchoolID, *parsed)
		if err != nil {
			return ListResult{}, err
		}
		if !parent.IsFolder {
			return ListResult{}, validationError("parent_id", "目标不是文件夹")
		}
	}
	items, err := s.repository.List(ctx, principal.SchoolID, parsed)
	if err != nil {
		return ListResult{}, err
	}
	refs := make([]ItemRef, 0, len(items))
	for _, item := range items {
		refs = append(refs, item.Ref())
	}
	breadcrumbs, err := s.breadcrumbs(ctx, principal.SchoolID, parsed)
	if err != nil {
		return ListResult{}, err
	}
	result := ListResult{ParentID: "", Breadcrumbs: breadcrumbs, Items: refs}
	if parsed != nil {
		result.ParentID = parsed.String()
	}
	return result, nil
}

func (s *Service) CreateFolder(ctx context.Context, principal auth.Principal, input CreateFolderInput) (ItemRef, error) {
	if err := validatePrincipal(principal); err != nil {
		return ItemRef{}, err
	}
	name, err := normalizeFolderName(input.Name)
	if err != nil {
		return ItemRef{}, err
	}
	parentID, err := parseOptionalID(input.ParentID, "parent_id")
	if err != nil {
		return ItemRef{}, err
	}
	if err := s.ensureFolder(ctx, principal.SchoolID, parentID); err != nil {
		return ItemRef{}, err
	}
	exists, err := s.repository.NameExists(ctx, principal.SchoolID, parentID, name)
	if err != nil {
		return ItemRef{}, err
	}
	if exists {
		return ItemRef{}, ErrConflict
	}
	item := Item{
		CreatorID: principal.UserID.String(),
		Name:      name,
		IsFolder:  true,
	}
	if parentID != nil {
		item.ParentID = parentID.String()
	}
	created, err := s.repository.Create(ctx, principal.SchoolID, item)
	if err != nil {
		return ItemRef{}, err
	}
	return created.Ref(), nil
}

func (s *Service) Upload(ctx context.Context, principal auth.Principal, parentID, name string, data []byte) (ItemRef, error) {
	if err := validatePrincipal(principal); err != nil {
		return ItemRef{}, err
	}
	if s.store == nil {
		return ItemRef{}, validationError("file", "云盘存储不可用")
	}
	parsedParent, err := parseOptionalID(parentID, "parent_id")
	if err != nil {
		return ItemRef{}, err
	}
	if err := s.ensureFolder(ctx, principal.SchoolID, parsedParent); err != nil {
		return ItemRef{}, err
	}
	stored, err := s.store.Save(principal.SchoolID, principal.UserID, name, data)
	if err != nil {
		return ItemRef{}, err
	}
	item := Item{
		CreatorID:   principal.UserID.String(),
		Name:        stored.Name,
		MIME:        stored.MIME,
		Kind:        stored.Kind,
		SizeBytes:   stored.SizeBytes,
		StoragePath: stored.StoragePath,
		SHA256:      stored.SHA256,
	}
	if parsedParent != nil {
		item.ParentID = parsedParent.String()
	}
	created, err := s.repository.Create(ctx, principal.SchoolID, item)
	if err != nil {
		_ = s.store.Remove(stored.StoragePath)
		return ItemRef{}, err
	}
	return created.Ref(), nil
}

// OpenFile 打开云盘文件，供下载接口流式返回。
func (s *Service) OpenFile(ctx context.Context, principal auth.Principal, id uuid.UUID) (Item, io.ReadCloser, error) {
	if err := validatePrincipal(principal); err != nil {
		return Item{}, nil, err
	}
	item, err := s.repository.Get(ctx, principal.SchoolID, id)
	if err != nil {
		return Item{}, nil, err
	}
	if item.IsFolder {
		return Item{}, nil, validationError("id", "文件夹不能下载")
	}
	if s.store == nil {
		return Item{}, nil, ErrNotFound
	}
	file, err := s.store.Open(item.StoragePath)
	if err != nil {
		return Item{}, nil, err
	}
	return item, file, nil
}

// Delete 递归软删除文件夹及其内容，并清理磁盘文件。
func (s *Service) Delete(ctx context.Context, principal auth.Principal, id uuid.UUID) (int, error) {
	if err := validatePrincipal(principal); err != nil {
		return 0, err
	}
	root, err := s.repository.Get(ctx, principal.SchoolID, id)
	if err != nil {
		return 0, err
	}
	collected := make([]Item, 0, 8)
	queue := []Item{root}
	visited := map[string]bool{}
	for len(queue) > 0 && len(collected) < maxDeleteDepth {
		current := queue[0]
		queue = queue[1:]
		if visited[current.ID] {
			continue
		}
		visited[current.ID] = true
		collected = append(collected, current)
		if !current.IsFolder {
			continue
		}
		children, err := s.repository.List(ctx, principal.SchoolID, uuidPtr(current.ID))
		if err != nil {
			return 0, err
		}
		queue = append(queue, children...)
	}
	for _, item := range collected {
		itemID, err := uuid.Parse(item.ID)
		if err != nil {
			return 0, err
		}
		if err := s.repository.MarkDeleted(ctx, principal.SchoolID, itemID); err != nil {
			return 0, err
		}
		if !item.IsFolder && s.store != nil {
			_ = s.store.Remove(item.StoragePath)
		}
	}
	return len(collected), nil
}

// ReadForAI 读取云盘文件内容，供 AI 工作台把云盘文件转换为对话附件。
func (s *Service) ReadForAI(ctx context.Context, principal auth.Principal, id uuid.UUID) (ReadFile, error) {
	if err := validatePrincipal(principal); err != nil {
		return ReadFile{}, err
	}
	item, file, err := s.OpenFile(ctx, principal, id)
	if err != nil {
		return ReadFile{}, err
	}
	defer file.Close()
	if item.SizeBytes > MaxFileBytes {
		return ReadFile{}, validationError("file", "文件超过 50 MiB 限制")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return ReadFile{}, err
	}
	if len(data) > MaxFileBytes {
		return ReadFile{}, validationError("file", "文件超过 50 MiB 限制")
	}
	return ReadFile{Name: item.Name, MIME: item.MIME, Data: data}, nil
}

// ImportFromAI 把 AI 对话附件同步到云盘根目录；相同内容的文件按 SHA256 去重。
func (s *Service) ImportFromAI(ctx context.Context, principal auth.Principal, name string, data []byte) error {
	if err := validatePrincipal(principal); err != nil {
		return err
	}
	if s.store == nil {
		return validationError("file", "云盘存储不可用")
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	if _, found, err := s.repository.FindBySHA(ctx, principal.SchoolID, hash); err != nil {
		return err
	} else if found {
		return nil
	}
	stored, err := s.store.Save(principal.SchoolID, principal.UserID, name, data)
	if err != nil {
		return err
	}
	_, err = s.repository.Create(ctx, principal.SchoolID, Item{
		CreatorID:   principal.UserID.String(),
		Name:        stored.Name,
		MIME:        stored.MIME,
		Kind:        stored.Kind,
		SizeBytes:   stored.SizeBytes,
		StoragePath: stored.StoragePath,
		SHA256:      stored.SHA256,
	})
	if err != nil {
		_ = s.store.Remove(stored.StoragePath)
		return err
	}
	return nil
}

func (s *Service) breadcrumbs(ctx context.Context, schoolID uuid.UUID, parentID *uuid.UUID) ([]Breadcrumb, error) {
	result := make([]Breadcrumb, 0, 4)
	current := parentID
	for depth := 0; current != nil && depth < maxBreadcrumbDepth; depth++ {
		item, err := s.repository.Get(ctx, schoolID, *current)
		if err != nil {
			return nil, err
		}
		result = append([]Breadcrumb{{ID: item.ID, Name: item.Name}}, result...)
		if item.ParentID == "" {
			break
		}
		parsed, err := uuid.Parse(item.ParentID)
		if err != nil {
			break
		}
		current = &parsed
	}
	return result, nil
}

func (s *Service) ensureFolder(ctx context.Context, schoolID uuid.UUID, parentID *uuid.UUID) error {
	if parentID == nil {
		return nil
	}
	parent, err := s.repository.Get(ctx, schoolID, *parentID)
	if err != nil {
		return err
	}
	if !parent.IsFolder {
		return validationError("parent_id", "目标不是文件夹")
	}
	return nil
}

func validatePrincipal(principal auth.Principal) error {
	if principal.UserID == uuid.Nil || principal.SchoolID == uuid.Nil {
		return ErrForbidden
	}
	if principal.Role != auth.RoleTeacher && principal.Role != auth.RoleAdmin {
		return ErrForbidden
	}
	return nil
}

func parseOptionalID(value, field string) (*uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil, validationError(field, "ID格式不正确")
	}
	return &parsed, nil
}

func normalizeFolderName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", validationError("name", "文件夹名称包含无效字符")
	}
	length := utf8.RuneCountInString(value)
	if length < 1 || length > maxFolderNameRunes {
		return "", validationError("name", "文件夹名称必须为1-120个字符")
	}
	if value == "." || value == ".." {
		return "", validationError("name", "文件夹名称不合法")
	}
	for _, char := range value {
		if unicode.IsControl(char) || char == '/' || char == '\\' {
			return "", validationError("name", "文件夹名称不能包含控制字符或斜杠")
		}
	}
	return value, nil
}

func uuidPtr(value string) *uuid.UUID {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &parsed
}
