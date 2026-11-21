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
	ErrIndexing   = errors.New("cloud search is not available")
)

// 云盘文件的向量索引状态。
const (
	IndexPending     = "pending"
	IndexProcessing  = "processing"
	IndexReady       = "ready"
	IndexUnsupported = "unsupported"
	IndexFailed      = "failed"
)

// 可自动抽取文本并向量化的格式；与 document 包的解析能力保持一致。
const (
	indexMaxFileBytes = 50 << 20
	searchDefaultHits = 8
	searchMaxHits     = 20
	searchChunkLimit  = 20000
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
	ID            string
	ParentID      string
	CreatorID     string
	Name          string
	IsFolder      bool
	MIME          string
	Kind          string
	SizeBytes     int64
	StoragePath   string
	SHA256        string
	IndexStatus   string
	IndexError    string
	IndexProvider string
	ChunkCount    int
	IndexedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ItemRef 是返回给前端的云盘节点摘要。
type ItemRef struct {
	ID            string     `json:"id"`
	ParentID      string     `json:"parent_id"`
	Name          string     `json:"name"`
	IsFolder      bool       `json:"is_folder"`
	MIME          string     `json:"mime"`
	Kind          string     `json:"kind"`
	SizeBytes     int64      `json:"size_bytes"`
	CreatorID     string     `json:"creator_id"`
	IndexStatus   string     `json:"index_status"`
	IndexError    string     `json:"index_error"`
	IndexProvider string     `json:"index_provider"`
	ChunkCount    int        `json:"chunk_count"`
	IndexedAt     *time.Time `json:"indexed_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (i Item) Ref() ItemRef {
	return ItemRef{
		ID:            i.ID,
		ParentID:      i.ParentID,
		Name:          i.Name,
		IsFolder:      i.IsFolder,
		MIME:          i.MIME,
		Kind:          i.Kind,
		SizeBytes:     i.SizeBytes,
		CreatorID:     i.CreatorID,
		IndexStatus:   i.IndexStatus,
		IndexError:    i.IndexError,
		IndexProvider: i.IndexProvider,
		ChunkCount:    i.ChunkCount,
		IndexedAt:     i.IndexedAt,
		CreatedAt:     i.CreatedAt,
		UpdatedAt:     i.UpdatedAt,
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

// BatchDeleteInput 是批量删除云盘条目的请求体。
type BatchDeleteInput struct {
	IDs []string `json:"ids"`
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

// ChunkRecord 是写入 cloud_chunks 的文本分块与向量。
type ChunkRecord struct {
	ItemID     string
	ChunkIndex int
	Content    string
	Embedding  []float32
	Provider   string
	CharCount  int
}

// ChunkMatch 是检索时按文件归并的候选分块。
type ChunkMatch struct {
	ItemID     string
	ItemName   string
	ItemKind   string
	ChunkIndex int
	Content    string
	Embedding  []float32
	Provider   string
	Score      float64
}

// CloudHit 是返回给 AI 与前端的一条云盘检索结果。
type CloudHit struct {
	ItemID     string  `json:"item_id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
	Excerpt    string  `json:"excerpt"`
}

// SearchResult 是一次云盘语义检索的完整结果。
type SearchResult struct {
	Query          string     `json:"query"`
	Provider       string     `json:"provider"`
	Hits           []CloudHit `json:"hits"`
	ChunksScanned  int        `json:"chunks_scanned"`
	IndexAvailable bool       `json:"index_available"`
}

// IndexOutcome 是单个文件完成向量化后的摘要。
type IndexOutcome struct {
	ItemID     string `json:"item_id"`
	Status     string `json:"status"`
	Provider   string `json:"provider"`
	ChunkCount int    `json:"chunk_count"`
	Message    string `json:"message,omitempty"`
}
