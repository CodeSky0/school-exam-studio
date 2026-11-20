package drive

import (
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("cloud item not found")
	ErrForbidden  = errors.New("cloud drive forbidden")
	ErrConflict   = errors.New("cloud item conflict")
	ErrValidation = errors.New("cloud drive validation failed")
)

// ValidationError 表示可以被安全返回给前端的字段校验错误。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
func (e *ValidationError) Unwrap() error { return ErrValidation }

func validationError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// Item 是云盘节点的内部记录。
type Item struct {
	ID          string
	ParentID    string
	CreatorID   string
	Name        string
	IsFolder    bool
	MIME        string
	Kind        string
	SizeBytes   int64
	StoragePath string
	SHA256      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ItemRef 是返回给前端的云盘节点摘要。
type ItemRef struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parent_id"`
	Name      string    `json:"name"`
	IsFolder  bool      `json:"is_folder"`
	MIME      string    `json:"mime"`
	Kind      string    `json:"kind"`
	SizeBytes int64     `json:"size_bytes"`
	CreatorID string    `json:"creator_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (i Item) Ref() ItemRef {
	return ItemRef{
		ID:        i.ID,
		ParentID:  i.ParentID,
		Name:      i.Name,
		IsFolder:  i.IsFolder,
		MIME:      i.MIME,
		Kind:      i.Kind,
		SizeBytes: i.SizeBytes,
		CreatorID: i.CreatorID,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

// Breadcrumb 是当前文件夹从根到当前层的路径节点。
type Breadcrumb struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListResult 返回某个文件夹的直接子节点与面包屑。
type ListResult struct {
	ParentID    string       `json:"parent_id"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs"`
	Items       []ItemRef    `json:"items"`
}

// CreateFolderInput 是新建文件夹的请求体。
type CreateFolderInput struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
}

// ReadFile 是供 AI 附件选择的云盘文件快照。
type ReadFile struct {
	Name string
	MIME string
	Data []byte
}

// StoredFile 是落盘后的文件元数据。
type StoredFile struct {
	Name        string
	MIME        string
	Kind        string
	SizeBytes   int64
	StoragePath string
	SHA256      string
}
