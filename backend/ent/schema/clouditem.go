package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CloudItem 是学校云盘中的文件夹或文件节点。
type CloudItem struct {
	ent.Schema
}

func (CloudItem) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (CloudItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("creator_id", uuid.UUID{}),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.String("name").MaxLen(255),
		field.Bool("is_folder").Default(false),
		field.String("mime").MaxLen(120).Default(""),
		field.String("kind").MaxLen(20).Default(""),
		field.Int64("size_bytes").Default(0),
		field.String("storage_path").MaxLen(500).Default(""),
		field.String("sha256").MaxLen(64).Default(""),
		// 云盘文件上传后自动进入向量化队列，状态供前端与 AI 检索使用。
		field.String("index_status").MaxLen(20).Default("pending"),
		field.String("index_error").MaxLen(500).Default(""),
		field.String("index_provider").MaxLen(80).Default(""),
		field.Int("chunk_count").Default(0),
		field.Time("indexed_at").Optional().Nillable(),
	}
}

func (CloudItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "parent_id", "created_at"),
		index.Fields("school_id", "sha256"),
	}
}
