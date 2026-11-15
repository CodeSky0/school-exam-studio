package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SchoolAISetting 保存学校级共享 AI 模型配置。API Key 以密文存储，仅后端解密使用。
type SchoolAISetting struct {
	ent.Schema
}

func (SchoolAISetting) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (SchoolAISetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("provider").MaxLen(30).Default("custom"),
		field.String("base_url").MaxLen(500).Default(""),
		field.String("api_key").Sensitive().Default(""),
		field.JSON("models", []string{}).Default([]string{}),
		field.String("default_model").MaxLen(120).Default(""),
		field.Bool("enabled").Default(false),
		field.UUID("updated_by", uuid.UUID{}).Optional().Nillable(),
	}
}

func (SchoolAISetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id").Unique(),
	}
}
