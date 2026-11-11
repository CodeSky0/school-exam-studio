package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{TenantMixin{}}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("role").NotEmpty().MaxLen(20),
		field.String("name").NotEmpty().MaxLen(50),
		field.String("subject").Optional().Default("").MaxLen(30),
		field.String("email").NotEmpty().MaxLen(320),
		field.String("password_hash").Sensitive().NotEmpty(),
		field.String("status").NotEmpty().Default("active").MaxLen(20),
		field.Int("session_version").Default(1).Min(1),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("school", School.Type).Ref("users").Field("school_id").Unique().Required(),
		edge.To("questions", Question.Type),
		edge.To("papers", Paper.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("school_id", "email").Unique(),
	}
}
