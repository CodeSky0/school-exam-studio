package drive

import (
	"context"
	"time"

	"github.com/google/uuid"
	"school-exam-studio/backend/ent"
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

func itemFromEnt(entity *ent.CloudItem) Item {
	item := Item{
		ID:          entity.ID.String(),
		CreatorID:   entity.CreatorID.String(),
		Name:        entity.Name,
		IsFolder:    entity.IsFolder,
		MIME:        entity.Mime,
		Kind:        entity.Kind,
		SizeBytes:   entity.SizeBytes,
		StoragePath: entity.StoragePath,
		SHA256:      entity.Sha256,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
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
