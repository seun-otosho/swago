// Package schema holds swago's core Ent schemas. A project re-exposes each one in its own
// ent/schema package by embedding it, e.g. `type Page struct{ schema.Page }`, so the
// generated client contains core and project entities together.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/seun-otosho/swago/tenant"
)

// Page is the base entity of the page tree. Page-type fields live in the JSON data
// column; a page type needing queryable columns links its own entity to this one.
type Page struct{ ent.Schema }

func (Page) Mixin() []ent.Mixin { return []ent.Mixin{tenant.Mixin{}} }

func (Page) Fields() []ent.Field {
	return []ent.Field{
		field.String("path").NotEmpty().Comment("treebeard materialized path, 4 base-36 chars per level"),
		field.Int("depth").Positive(),
		field.Int("numchild").Default(0).NonNegative(),
		field.String("title").NotEmpty(),
		field.String("slug").NotEmpty(),
		field.String("url_path").Comment("slugs of the ancestors and the page itself, e.g. /blog/hello/"),
		field.String("type").NotEmpty().Comment("registered page type name"),
		field.JSON("data", map[string]any{}).Optional(),
	}
}

func (Page) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "path").Unique(),
		index.Fields("tenant_id", "url_path").Unique(), // also makes sibling slugs unique
	}
}
