package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type PaperQuestion struct {
	ent.Schema
}

func (PaperQuestion) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (PaperQuestion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("paper_id", uuid.UUID{}),
		field.UUID("question_id", uuid.UUID{}),
		field.Int("order_index").Min(0),
		field.String("assigned_score").SchemaType(map[string]string{dialect.Postgres: "numeric(8,2)"}),
	}
}

func (PaperQuestion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("school", School.Type).Ref("paper_questions").Field("school_id").Unique().Required(),
		edge.From("paper", Paper.Type).Ref("paper_questions").Field("paper_id").Unique().Required(),
		edge.From("question", Question.Type).Ref("paper_questions").Field("question_id").Unique().Required(),
	}
}

func (PaperQuestion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "paper_id", "order_index"),
		index.Fields("paper_id", "order_index").Unique(),
		index.Fields("paper_id", "question_id").Unique(),
	}
}
