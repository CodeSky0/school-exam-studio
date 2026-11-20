package drive

import (
	"context"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/ent/cloudchunk"
	"school-exam-studio/backend/ent/clouditem"
)

type Repository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) Create(ctx context.Context, schoolID uuid.UUID, item Item) (Item, error) {
	create := r.client.CloudItem.Create().
		SetSchoolID(schoolID).
		SetCreatorID(mustUUID(item.CreatorID)).
		SetName(item.Name).
		SetIsFolder(item.IsFolder).
		SetMime(item.MIME).
		SetKind(item.Kind).
		SetSizeBytes(item.SizeBytes).
		SetStoragePath(item.StoragePath).
		SetSha256(item.SHA256)
	if item.ParentID != "" {
		parentID, err := uuid.Parse(item.ParentID)
		if err != nil {
			return Item{}, validationError("parent_id", "文件夹ID格式不正确")
		}
		create.SetParentID(parentID)
	}
	status := item.IndexStatus
	if status == "" {
		if item.IsFolder {
			status = IndexUnsupported
		} else {
			status = IndexPending
		}
	}
	create.SetIndexStatus(status)
	entity, err := create.Save(ctx)
	if err != nil {
		return Item{}, err
	}
	return itemFromEnt(entity), nil
}

func (r *Repository) Get(ctx context.Context, schoolID, id uuid.UUID) (Item, error) {
	entity, err := r.client.CloudItem.Query().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.IDEQ(id),
		clouditem.DeletedAtIsNil(),
	).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	return itemFromEnt(entity), nil
}

func (r *Repository) List(ctx context.Context, schoolID uuid.UUID, parentID *uuid.UUID) ([]Item, error) {
	query := r.client.CloudItem.Query().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.DeletedAtIsNil(),
	)
	if parentID == nil {
		query.Where(clouditem.ParentIDIsNil())
	} else {
		query.Where(clouditem.ParentIDEQ(*parentID))
	}
	entities, err := query.Order(ent.Desc(clouditem.FieldIsFolder), ent.Asc(clouditem.FieldName)).All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(entities))
	for _, entity := range entities {
		items = append(items, itemFromEnt(entity))
	}
	return items, nil
}

// FindBySHA 用于 AI 附件自动入云盘时去重：同校已有相同内容文件时直接复用。
func (r *Repository) FindBySHA(ctx context.Context, schoolID uuid.UUID, sha string) (Item, bool, error) {
	if sha == "" {
		return Item{}, false, nil
	}
	entity, err := r.client.CloudItem.Query().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.Sha256EQ(sha),
		clouditem.IsFolderEQ(false),
		clouditem.DeletedAtIsNil(),
	).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return Item{}, false, nil
		}
		return Item{}, false, err
	}
	return itemFromEnt(entity), true, nil
}

// NameExists 检查同一目录下是否已有同名文件夹，避免生成两个难以区分的目录。
func (r *Repository) NameExists(ctx context.Context, schoolID uuid.UUID, parentID *uuid.UUID, name string) (bool, error) {
	query := r.client.CloudItem.Query().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.NameEQ(name),
		clouditem.DeletedAtIsNil(),
	)
	if parentID == nil {
		query.Where(clouditem.ParentIDIsNil())
	} else {
		query.Where(clouditem.ParentIDEQ(*parentID))
	}
	return query.Exist(ctx)
}

func (r *Repository) MarkDeleted(ctx context.Context, schoolID, id uuid.UUID) error {
	_, err := r.client.CloudItem.Update().
		Where(
			clouditem.SchoolIDEQ(schoolID),
			clouditem.IDEQ(id),
			clouditem.DeletedAtIsNil(),
		).
		SetDeletedAt(time.Now()).
		Save(ctx)
	return err
}

// UpdateIndexState 记录单个文件的向量化状态，供上传后的异步任务与重试使用。
func (r *Repository) UpdateIndexState(ctx context.Context, schoolID, id uuid.UUID, status, provider, message string, chunkCount int, indexedAt *time.Time) error {
	update := r.client.CloudItem.Update().
		Where(
			clouditem.SchoolIDEQ(schoolID),
			clouditem.IDEQ(id),
			clouditem.DeletedAtIsNil(),
		).
		SetIndexStatus(status).
		SetIndexError(message).
		SetIndexProvider(provider).
		SetChunkCount(chunkCount)
	if indexedAt != nil {
		update.SetIndexedAt(*indexedAt)
	}
	_, err := update.Save(ctx)
	if ent.IsNotFound(err) {
		return ErrNotFound
	}
	return err
}

// ReplaceChunks 在事务内替换某个文件的全部文本分块，并标记索引完成。
func (r *Repository) ReplaceChunks(ctx context.Context, schoolID, id uuid.UUID, chunks []ChunkRecord, provider string) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		_ = tx.Rollback()
		return cause
	}
	if _, err := tx.CloudChunk.Delete().Where(
		cloudchunk.SchoolIDEQ(schoolID),
		cloudchunk.CloudItemIDEQ(id),
	).Exec(ctx); err != nil {
		return rollback(err)
	}
	builders := make([]*ent.CloudChunkCreate, 0, len(chunks))
	for _, chunk := range chunks {
		builder := tx.CloudChunk.Create().
			SetSchoolID(schoolID).
			SetCloudItemID(id).
			SetChunkIndex(chunk.ChunkIndex).
			SetContent(chunk.Content).
			SetEmbedding(chunk.Embedding).
			SetProvider(provider).
			SetCharCount(chunk.CharCount)
		builders = append(builders, builder)
	}
	if len(builders) > 0 {
		if err := tx.CloudChunk.CreateBulk(builders...).Exec(ctx); err != nil {
			return rollback(err)
		}
	}
	now := time.Now()
	if _, err := tx.CloudItem.Update().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.IDEQ(id),
		clouditem.DeletedAtIsNil(),
	).SetIndexStatus(IndexReady).
		SetIndexError("").
		SetIndexProvider(provider).
		SetChunkCount(len(chunks)).
		SetIndexedAt(now).
		Save(ctx); err != nil {
		return rollback(err)
	}
	return tx.Commit()
}

// ClearChunks 删除文件的全部向量，用于删除文件或重新索引。
func (r *Repository) ClearChunks(ctx context.Context, schoolID, id uuid.UUID) error {
	_, err := r.client.CloudChunk.Delete().Where(
		cloudchunk.SchoolIDEQ(schoolID),
		cloudchunk.CloudItemIDEQ(id),
	).Exec(ctx)
	return err
}

// ListSearchableChunks 读取本校指定向量来源的全部分块，供进程内相似度排序。
// 单次上限保证大校也不会因异常数据拖垮请求。
func (r *Repository) ListSearchableChunks(ctx context.Context, schoolID uuid.UUID, provider string, limit int) ([]ChunkMatch, error) {
	entities, err := r.client.CloudChunk.Query().Where(
		cloudchunk.SchoolIDEQ(schoolID),
		cloudchunk.ProviderEQ(provider),
		cloudchunk.DeletedAtIsNil(),
	).Order(ent.Asc(cloudchunk.FieldID)).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 {
		return nil, nil
	}
	itemIDs := make([]uuid.UUID, 0, len(entities))
	seen := make(map[uuid.UUID]bool, len(entities))
	for _, entity := range entities {
		if !seen[entity.CloudItemID] {
			seen[entity.CloudItemID] = true
			itemIDs = append(itemIDs, entity.CloudItemID)
		}
	}
	items, err := r.client.CloudItem.Query().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.IDIn(itemIDs...),
		clouditem.IsFolderEQ(false),
		clouditem.DeletedAtIsNil(),
	).All(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]Item, len(items))
	for _, item := range items {
		names[item.ID] = itemFromEnt(item)
	}
	matches := make([]ChunkMatch, 0, len(entities))
	for _, entity := range entities {
		item, ok := names[entity.CloudItemID]
		if !ok {
			continue
		}
		matches = append(matches, ChunkMatch{
			ItemID:     entity.CloudItemID.String(),
			ItemName:   item.Name,
			ItemKind:   item.Kind,
			ChunkIndex: entity.ChunkIndex,
			Content:    entity.Content,
			Embedding:  entity.Embedding,
			Provider:   entity.Provider,
		})
	}
	return matches, nil
}

// ListFileIDs 返回本校全部可向量化的文件，用于切换向量模型后的全量重建。
func (r *Repository) ListFileIDs(ctx context.Context, schoolID uuid.UUID) ([]uuid.UUID, error) {
	entities, err := r.client.CloudItem.Query().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.IsFolderEQ(false),
		clouditem.DeletedAtIsNil(),
	).Select(clouditem.FieldID).All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(entities))
	for _, entity := range entities {
		ids = append(ids, entity.ID)
	}
	return ids, nil
}

// IndexStats 汇总本校云盘索引状态，供前端展示与运维排查。
func (r *Repository) IndexStats(ctx context.Context, schoolID uuid.UUID) (map[string]int, error) {
	entities, err := r.client.CloudItem.Query().Where(
		clouditem.SchoolIDEQ(schoolID),
		clouditem.IsFolderEQ(false),
		clouditem.DeletedAtIsNil(),
	).Select(clouditem.FieldIndexStatus).All(ctx)
	if err != nil {
		return nil, err
	}
	stats := map[string]int{}
	for _, entity := range entities {
		stats[entity.IndexStatus]++
	}
	return stats, nil
}

func itemFromEnt(entity *ent.CloudItem) Item {
	item := Item{
		ID:            entity.ID.String(),
		CreatorID:     entity.CreatorID.String(),
		Name:          entity.Name,
		IsFolder:      entity.IsFolder,
		MIME:          entity.Mime,
		Kind:          entity.Kind,
		SizeBytes:     entity.SizeBytes,
		StoragePath:   entity.StoragePath,
		SHA256:        entity.Sha256,
		IndexStatus:   entity.IndexStatus,
		IndexError:    entity.IndexError,
		IndexProvider: entity.IndexProvider,
		ChunkCount:    entity.ChunkCount,
		IndexedAt:     entity.IndexedAt,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
	if entity.ParentID != nil {
		item.ParentID = entity.ParentID.String()
	}
	return item
}

func mustUUID(value string) uuid.UUID {
	parsed, _ := uuid.Parse(value)
	return parsed
}
