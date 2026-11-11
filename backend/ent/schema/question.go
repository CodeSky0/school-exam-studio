package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Question struct {
	ent.Schema
}

func (Question) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (Question) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("creator_id", uuid.UUID{}),
		field.String("subject").NotEmpty().MaxLen(30),
		field.String("grade").NotEmpty().MaxLen(30),
		field.String("question_type").NotEmpty().MaxLen(30),
		field.Int("difficulty").Min(1).Max(5),
		field.JSON("tags", []string{}).Default([]string{}),
		field.Text("content").NotEmpty(),
		field.JSON("options", []Option{}).Default([]Option{}),
		field.Text("answer").Optional().Default(""),
		field.Text("explanation").Optional().Default(""),
		field.String("status").NotEmpty().Default("draft").MaxLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}),
	}
}

func (Question) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("school", School.Type).Ref("questions").Field("school_id").Unique().Required(),
		edge.From("creator", User.Type).Ref("questions").Field("creator_id").Unique().Required(),
		edge.To("paper_questions", PaperQuestion.Type),
	}
}

func (Question) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "subject", "grade", "question_type"),
		index.Fields("school_id", "creator_id", "status"),
	}
}
