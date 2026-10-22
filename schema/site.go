package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/seun-otosho/swago/tenant"
)

// Site maps an incoming request's hostname (and port) to a tenant and a root page, as in
// Wagtail's Site model.
//
// hostname+port are unique across every tenant, not just within one, because the tenant is
// not known until a site is resolved from them. Routing code looks sites up with
// tenant.AllowWithoutTenant (see package routing), the one documented escape hatch for this;
// every other Site query (listing a tenant's own sites in the admin, say) goes through the
// normal tenant.Mixin filter like any other entity.
type Site struct{ ent.Schema }

func (Site) Mixin() []ent.Mixin { return []ent.Mixin{tenant.Mixin{}} }

func (Site) Fields() []ent.Field {
	return []ent.Field{
		field.String("hostname").NotEmpty().Comment("no scheme, port or path"),
		field.Int("port").Default(80),
		field.String("site_name").Optional(),
		field.Bool("is_default_site").Default(false).
			Comment("fallback when no hostname matches. Not enforced unique by the schema " +
				"(see package routing): at most one should be set, across the whole install, " +
				"not per tenant, since it is picked before the tenant is known."),
	}
}

func (Site) Edges() []ent.Edge {
	return []ent.Edge{edge.To("root_page", Page.Type).Unique().Required()}
}

func (Site) Indexes() []ent.Index {
	// Global, not tenant-prefixed: see the comment on Site above.
	return []ent.Index{index.Fields("hostname", "port").Unique()}
}
