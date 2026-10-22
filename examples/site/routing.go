package site

import (
	"context"

	"github.com/seun-otosho/swago/examples/site/ent"
	entsite "github.com/seun-otosho/swago/examples/site/ent/site"
	"github.com/seun-otosho/swago/routing"
)

// Routing returns the site resolver backed by c.
func Routing(c *ent.Client) routing.Store { return routingStore{c} }

type routingStore struct{ c *ent.Client }

func toRoutingSite(r *ent.Site) routing.Site {
	return routing.Site{
		ID: r.ID, TenantID: r.TenantID, Hostname: r.Hostname, Port: r.Port,
		SiteName: r.SiteName, IsDefault: r.IsDefaultSite, RootPageID: r.Edges.RootPage.ID,
	}
}

func (s routingStore) ByHostname(ctx context.Context, hostname string) ([]routing.Site, error) {
	rows, err := s.c.Site.Query().Where(entsite.Hostname(hostname)).WithRootPage().All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]routing.Site, len(rows))
	for i, r := range rows {
		out[i] = toRoutingSite(r)
	}
	return out, nil
}

func (s routingStore) ByHostnamePort(ctx context.Context, hostname string, port int) (routing.Site, bool, error) {
	r, err := s.c.Site.Query().Where(entsite.Hostname(hostname), entsite.Port(port)).WithRootPage().Only(ctx)
	if ent.IsNotFound(err) {
		return routing.Site{}, false, nil
	}
	if err != nil {
		return routing.Site{}, false, err
	}
	return toRoutingSite(r), true, nil
}

// Default returns the lowest-id site marked is_default_site: the schema does not enforce
// at most one (see schema.Site), so more than one picks a deterministic row rather than
// an arbitrary one.
func (s routingStore) Default(ctx context.Context) (routing.Site, bool, error) {
	r, err := s.c.Site.Query().Where(entsite.IsDefaultSite(true)).Order(ent.Asc(entsite.FieldID)).WithRootPage().First(ctx)
	if ent.IsNotFound(err) {
		return routing.Site{}, false, nil
	}
	if err != nil {
		return routing.Site{}, false, err
	}
	return toRoutingSite(r), true, nil
}
