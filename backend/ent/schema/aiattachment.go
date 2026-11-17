package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AIAttachment 记录用户上传给 Agent 的图片或文档及其文本抽取结果。
type AIAttachment struct {
	ent.Schema
}

func (AIAttachment) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (AIAttachment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("creator_id", uuid.UUID{}),
		field.UUID("conversation_id", uuid.UUID{}).Optional().Nillable(),
		field.String("name").MaxLen(255),
		field.String("mime").MaxLen(120),
		field.String("kind").MaxLen(20),
		field.Int64("size_bytes"),
		field.String("storage_path").MaxLen(500).Default(""),
		field.String("sha256").MaxLen(64).Default(""),
		field.Text("extracted_text").Default(""),
		field.Int("text_chars").Default(0),
		field.String("status").MaxLen(20).Default("uploaded"),
		field.Text("error").Default(""),
	}
}

func (AIAttachment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "creator_id", "created_at"),
		index.Fields("school_id", "sha256"),
	}
}
