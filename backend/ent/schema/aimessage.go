package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AIMessage 保存对话中的一条用户或助手消息，以及该轮的全部工具调用。
type AIMessage struct {
	ent.Schema
}

func (AIMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (AIMessage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("conversation_id", uuid.UUID{}),
		field.String("role").MaxLen(20).Default("user"),
		field.Text("content").Default(""),
		field.String("status").MaxLen(20).Default("succeeded"),
		field.String("provider").MaxLen(20).Default("local"),
		field.String("model").MaxLen(80).Default(""),
		field.JSON("tool_calls", []map[string]any{}).Default([]map[string]any{}),
		field.JSON("attachments", []map[string]any{}).Default([]map[string]any{}),
		field.UUID("run_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("error").Default(""),
	}
}

func (AIMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "conversation_id", "created_at"),
	}
}
