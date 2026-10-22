// Package site is the example project: the shape `swago new` scaffolds.
package site

import (
	"context"

	entgo "entgo.io/ent"

	swagodb "github.com/seun-otosho/swago/db"
	"github.com/seun-otosho/swago/examples/site/ent"
	"github.com/seun-otosho/swago/examples/site/ent/intercept"
	_ "github.com/seun-otosho/swago/examples/site/ent/runtime" // registers the tenant hooks and interceptors
	"github.com/seun-otosho/swago/tenant"
)

func init() {
	tenant.QueryAdapter = func(q entgo.Query) (tenant.Wherer, error) { return intercept.NewQuery(q) }
}

// Open connects to url and applies the schema (dev auto-migrate).
func Open(ctx context.Context, url string) (*ent.Client, error) {
	drv, err := swagodb.Open(url)
	if err != nil {
		return nil, err
	}
	c := ent.NewClient(ent.Driver(drv))
	if err := c.Schema.Create(tenant.AllowWithoutTenant(ctx), tenant.CompositeFKs); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
