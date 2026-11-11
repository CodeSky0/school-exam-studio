package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type School struct {
	ent.Schema
}

func (School) Mixin() []ent.Mixin {
	return []ent.Mixin{BaseMixin{}}
}

func (School) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().MaxLen(200),
		field.String("school_code").NotEmpty().MaxLen(8).Unique(),
	}
}

func (School) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type),
		edge.To("questions", Question.Type),
		edge.To("papers", Paper.Type),
		edge.To("paper_questions", PaperQuestion.Type),
	}
}
