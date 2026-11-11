package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Paper struct {
	ent.Schema
}

func (Paper) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (Paper) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("creator_id", uuid.UUID{}),
		field.String("title").NotEmpty().MaxLen(200),
		field.String("subject").NotEmpty().MaxLen(30),
		field.String("grade").NotEmpty().MaxLen(30),
		field.String("total_score").Default("0.00").SchemaType(map[string]string{dialect.Postgres: "numeric(8,2)"}),
		field.Int("duration_minutes").Min(1).Max(600),
	}
}

func (Paper) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("school", School.Type).Ref("papers").Field("school_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("papers").Field("creator_id").Unique().Required(),
		edge.To("paper_questions", PaperQuestion.Type),
	}
}

func (Paper) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "creator_id", "updated_at"),
	}
}
