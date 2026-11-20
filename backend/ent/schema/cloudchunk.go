package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CloudChunk 是云盘文件文本分块及其向量，供 AI 语义检索使用。
type CloudChunk struct {
	ent.Schema
}

func (CloudChunk) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (CloudChunk) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("cloud_item_id", uuid.UUID{}),
		field.Int("chunk_index").Default(0),
		field.String("content").MaxLen(8000),
		field.JSON("embedding", []float32{}),
		field.String("provider").MaxLen(80).Default(""),
		field.Int("char_count").Default(0),
	}
}

func (CloudChunk) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "cloud_item_id", "chunk_index"),
		index.Fields("school_id", "provider"),
	}
}