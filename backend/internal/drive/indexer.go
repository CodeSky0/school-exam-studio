package drive

import (
	"context"
	"errors"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/document"
	"school-exam-studio/backend/internal/embedding"
)

const (
	indexWorkers      = 2
	indexQueueSize    = 512
	indexTaskTimeout  = 2 * time.Minute
	indexEmbedBatch   = 16
	indexErrorLength  = 480
	searchScoreFloor  = 0.05
	searchHitsPerFile = 2
	searchExcerptRune = 260
)

type indexTask struct {
	schoolID uuid.UUID
	itemID   uuid.UUID
}

// ContentIndexer 是云盘服务依赖的向量化与检索能力。
// 使用接口便于上传流程在不启动后台任务时保持可测试。
type ContentIndexer interface {
	Enqueue(schoolID, itemID uuid.UUID)
	Process(ctx context.Context, schoolID, itemID uuid.UUID) (IndexOutcome, error)
	Search(ctx context.Context, schoolID uuid.UUID, query string, limit int) (SearchResult, error)
}

// Indexer 在后台把上传到云盘的文件解析、分块并写入向量索引。
type Indexer struct {
	repository *Repository
	store      *FileStore
	embedder   embedding.Embedder
	logger     *log.Logger
	queue      chan indexTask
}

func NewIndexer(repository *Repository, store *FileStore, embedder embedding.Embedder, logger *log.Logger) *Indexer {
	if embedder == nil {
		embedder = embedding.NewLocal(256)
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Indexer{
		repository: repository,
		store:      store,
		embedder:   embedder,
		logger:     logger,
		queue:      make(chan indexTask, indexQueueSize),
	}
}

// Provider 返回当前向量来源标识，前端与运维可用它判断是否需要重建索引。
func (i *Indexer) Provider() string {
	if i == nil || i.embedder == nil {
		return ""
	}
	return i.embedder.Name()
}

// Start 启动后台工作协程；ctx 取消后停止接收新任务。
func (i *Indexer) Start(ctx context.Context) {
	if i == nil || i.repository == nil {
		return
	}
	for worker := 0; worker < indexWorkers; worker++ {
		go i.work(ctx)
	}
}

// Enqueue 把文件放入索引队列。队列满时丢弃任务并记录日志，
// 文件会保持 pending 状态，可通过“重建索引”重新入队，不会阻塞上传。
func (i *Indexer) Enqueue(schoolID, itemID uuid.UUID) {
	if i == nil {
		return
	}
	if schoolID == uuid.Nil || itemID == uuid.Nil {
		return
	}
	select {
	case i.queue <- indexTask{schoolID: schoolID, itemID: itemID}:
	default:
		i.logger.Printf("cloud index queue full, item %s stays pending", itemID)
	}
}

func (i *Indexer) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-i.queue:
			taskCtx, cancel := context.WithTimeout(ctx, indexTaskTimeout)
			outcome, err := i.Process(taskCtx, task.schoolID, task.itemID)
			cancel()
			if err != nil {
				i.logger.Printf("cloud index item %s failed: %v", task.itemID, err)
				// 任务超时或数据库错误时进程内的 ctx 已取消，用独立上下文落状态。
				stateCtx, stateCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = i.repository.UpdateIndexState(stateCtx, task.schoolID, task.itemID, IndexFailed, i.Provider(), "索引任务中断："+truncateIndexError(err.Error()), 0, nil)
				stateCancel()
				continue
			}
			if outcome.Status == IndexFailed {
				i.logger.Printf("cloud index item %s failed: %s", task.itemID, outcome.Message)
			}
		}
	}
}

// Process 同步完成单个文件的解析、分块与向量写入，供后台任务与手动重建共用。
// 解析失败会记录为 failed 或 unsupported 状态并通过 IndexOutcome 返回，不产生 error。
func (i *Indexer) Process(ctx context.Context, schoolID, itemID uuid.UUID) (IndexOutcome, error) {
	if i == nil || i.repository == nil {
		return IndexOutcome{}, ErrIndexing
	}
	outcome := IndexOutcome{ItemID: itemID.String(), Provider: i.Provider()}
	item, err := i.repository.Get(ctx, schoolID, itemID)
	if err != nil {
		return outcome, err
	}
	outcome.ItemID = item.ID
	if item.IsFolder {
		return i.mark(ctx, outcome, schoolID, itemID, IndexUnsupported, "文件夹无需向量化")
	}
	if err := i.repository.UpdateIndexState(ctx, schoolID, itemID, IndexProcessing, i.Provider(), "", 0, nil); err != nil {
		return outcome, err
	}
	if i.store == nil || i.embedder == nil {
		return i.mark(ctx, outcome, schoolID, itemID, IndexFailed, "云盘索引服务未启用")
	}
	if item.SizeBytes > indexMaxFileBytes {
		return i.mark(ctx, outcome, schoolID, itemID, IndexUnsupported, "文件超过 50 MiB，未建立向量索引")
	}
	file, err := i.store.Open(item.StoragePath)
	if err != nil {
		return i.mark(ctx, outcome, schoolID, itemID, IndexFailed, "无法读取云盘文件")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, indexMaxFileBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return i.mark(ctx, outcome, schoolID, itemID, IndexFailed, "无法读取云盘文件")
	}
	if len(data) > indexMaxFileBytes {
		return i.mark(ctx, outcome, schoolID, itemID, IndexUnsupported, "文件超过 50 MiB，未建立向量索引")
	}
	extracted, err := document.Extract(item.Name, item.MIME, data)
	if err != nil {
		switch {
		case errors.Is(err, document.ErrUnsupported):
			return i.mark(ctx, outcome, schoolID, itemID, IndexUnsupported, "该格式暂不支持向量化，请转换为 PDF、Word 或文本")
		case errors.Is(err, document.ErrEmpty):
			return i.mark(ctx, outcome, schoolID, itemID, IndexUnsupported, "未提取到可索引文本，扫描版 PDF 可先转换为可复制文本")
		default:
			return i.mark(ctx, outcome, schoolID, itemID, IndexFailed, "解析文档文本失败")
		}
	}
	chunks := splitContent(extracted.Text)
	if len(chunks) == 0 {
		return i.mark(ctx, outcome, schoolID, itemID, IndexUnsupported, "未提取到可索引文本")
	}
	records := make([]ChunkRecord, 0, len(chunks))
	for start := 0; start < len(chunks); start += indexEmbedBatch {
		end := start + indexEmbedBatch
		if end > len(chunks) {
			end = len(chunks)
		}
		vectors, err := i.embedder.Embed(ctx, chunks[start:end])
		if err != nil {
			return i.mark(ctx, outcome, schoolID, itemID, IndexFailed, "生成文本向量失败："+truncateIndexError(err.Error()))
		}
		if len(vectors) != end-start {
			return i.mark(ctx, outcome, schoolID, itemID, IndexFailed, "向量数量与文本分块不一致")
		}
		for offset, vector := range vectors {
			position := start + offset
			records = append(records, ChunkRecord{
				ItemID:     item.ID,
				ChunkIndex: position,
				Content:    chunks[position],
				Embedding:  vector,
				Provider:   i.Provider(),
				CharCount:  len([]rune(chunks[position])),
			})
		}
	}
	if err := i.repository.ReplaceChunks(ctx, schoolID, itemID, records, i.Provider()); err != nil {
		return outcome, err
	}
	outcome.Status = IndexReady
	outcome.ChunkCount = len(records)
	return outcome, nil
}

// mark 记录未完成索引的原因，并返回带状态的 outcome，供前端展示。
func (i *Indexer) mark(ctx context.Context, outcome IndexOutcome, schoolID, itemID uuid.UUID, status, message string) (IndexOutcome, error) {
	message = truncateIndexError(message)
	if err := i.repository.UpdateIndexState(ctx, schoolID, itemID, status, i.Provider(), message, 0, nil); err != nil {
		return outcome, err
	}
	outcome.Status = status
	outcome.Message = message
	return outcome, nil
}

// Search 对查询做向量化，并在本校已索引分块中按余弦相似度排序。
func (i *Indexer) Search(ctx context.Context, schoolID uuid.UUID, query string, limit int) (SearchResult, error) {
	result := SearchResult{Query: strings.TrimSpace(query), IndexAvailable: false}
	if i == nil || i.repository == nil || i.embedder == nil {
		return result, ErrIndexing
	}
	if result.Query == "" {
		return result, validationError("query", "请输入检索关键词")
	}
	if len([]rune(result.Query)) > 200 {
		return result, validationError("query", "检索关键词不能超过200个字符")
	}
	if limit <= 0 {
		limit = searchDefaultHits
	}
	if limit > searchMaxHits {
		limit = searchMaxHits
	}
	vectors, err := i.embedder.Embed(ctx, []string{result.Query})
	if err != nil || len(vectors) != 1 {
		return result, ErrIndexing
	}
	matches, err := i.repository.ListSearchableChunks(ctx, schoolID, i.embedder.Name(), searchChunkLimit)
	if err != nil {
		return result, err
	}
	result.Provider = i.embedder.Name()
	result.IndexAvailable = true
	result.ChunksScanned = len(matches)
	if len(matches) == 0 {
		return result, nil
	}
	for index := range matches {
		matches[index].Score = embedding.Cosine(vectors[0], matches[index].Embedding)
	}
	result.Hits = rankMatches(matches, limit)
	return result, nil
}

// rankMatches 按相似度排序并归并结果：每个文件最多保留两个片段，
// 低于阈值的分块直接丢弃，避免无关内容进入 AI 上下文。
func rankMatches(matches []ChunkMatch, limit int) []CloudHit {
	sort.SliceStable(matches, func(a, b int) bool { return matches[a].Score > matches[b].Score })
	perFile := make(map[string]int, limit)
	hits := make([]CloudHit, 0, limit)
	for _, match := range matches {
		if match.Score < searchScoreFloor {
			break
		}
		if perFile[match.ItemID] >= searchHitsPerFile {
			continue
		}
		perFile[match.ItemID]++
		hits = append(hits, CloudHit{
			ItemID:     match.ItemID,
			Name:       match.ItemName,
			Kind:       match.ItemKind,
			ChunkIndex: match.ChunkIndex,
			Score:      roundScore(match.Score),
			Excerpt:    excerptText(match.Content, searchExcerptRune),
		})
		if len(hits) >= limit {
			break
		}
	}
	return hits
}

func truncateIndexError(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= indexErrorLength {
		return string(runes)
	}
	return string(runes[:indexErrorLength]) + "…"
}

func excerptText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func roundScore(value float64) float64 {
	return float64(int(value*10000+0.5)) / 10000
}
