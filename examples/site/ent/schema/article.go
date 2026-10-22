package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/seun-otosho/swago/tenant"
)

// Article is a project page type that needs a queryable column, so it extends a core page.
type Article struct{ ent.Schema }

func (Article) Mixin() []ent.Mixin { return []ent.Mixin{tenant.Mixin{}} }

func (Article) Fields() []ent.Field {
	return []ent.Field{field.String("author").NotEmpty()}
}

func (Article) Edges() []ent.Edge {
	return []ent.Edge{edge.To("page", Page.Type).Unique().Required().
		Annotations(entsql.OnDelete(entsql.Cascade))}
}
