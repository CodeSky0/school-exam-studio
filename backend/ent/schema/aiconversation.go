package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AIConversation 是一个可长期保存的 AI Agent 对话会话。
type AIConversation struct {
	ent.Schema
}

func (AIConversation) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (AIConversation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("creator_id", uuid.UUID{}),
		field.String("title").MaxLen(120).Default("新对话"),
		field.String("status").MaxLen(20).Default("active"),
		field.Bool("pinned").Default(false),
		field.Int("message_count").Default(0),
		field.Time("last_message_at").Default(time.Now),
	}
}

func (AIConversation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "creator_id", "pinned", "last_message_at"),
	}
}
