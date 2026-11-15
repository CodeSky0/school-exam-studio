package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AIRun 记录一次 AI Agent 的完整运行：目标、意图、计划、步骤、结果与引擎信息。
type AIRun struct {
	ent.Schema
}

func (AIRun) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (AIRun) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("creator_id", uuid.UUID{}),
		field.Text("goal"),
		field.String("intent").MaxLen(40).Default("unknown"),
		field.String("status").MaxLen(20).Default("succeeded"),
		field.String("provider").MaxLen(20).Default("local"),
		field.String("model").MaxLen(80).Default(""),
		field.JSON("plan", []map[string]any{}).Default([]map[string]any{}),
		field.JSON("steps", []map[string]any{}).Default([]map[string]any{}),
		field.JSON("result", map[string]any{}).Default(map[string]any{}),
		field.Text("error").Optional().Default(""),
	}
}

func (AIRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "creator_id", "created_at"),
		index.Fields("school_id", "status"),
	}
}
