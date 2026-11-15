package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AIAuditLog 记录 AI 对题库产生的每一次可追溯变更。
type AIAuditLog struct {
	ent.Schema
}

func (AIAuditLog) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (AIAuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("actor_id", uuid.UUID{}),
		field.UUID("run_id", uuid.UUID{}).Optional().Nillable(),
		field.String("action").MaxLen(60),
		field.String("target_type").MaxLen(30),
		field.UUID("target_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("detail", map[string]any{}).Default(map[string]any{}),
	}
}

func (AIAuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "created_at"),
		index.Fields("school_id", "actor_id", "created_at"),
	}
}
