package site_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seun-otosho/swago/examples/site"
	"github.com/seun-otosho/swago/examples/site/ent"
	"github.com/seun-otosho/swago/routing"
	"github.com/seun-otosho/swago/tenant"
)

func mkSite(c *ent.Client, ctx context.Context, hostname string, port int, dflt bool, root *ent.Page) *ent.Site {
	return c.Site.Create().SetHostname(hostname).SetPort(port).SetIsDefaultSite(dflt).SetRootPage(root).SaveX(ctx)
}

func TestSiteResolution(t *testing.T) {
	eachBackend(t, func(t *testing.T, url func(string) string) {
		c := newClient(t, url(""))
		a, b := tenant.With(context.Background(), 1), tenant.With(context.Background(), 2)

		homeA := mkPage(c, a, "0001", "home-a")
		mkSite(c, a, "a.example", 80, false, homeA)

		homeB := mkPage(c, b, "0001", "home-b")
		fallback := mkSite(c, b, "fallback.example", 80, true, homeB)

		store := site.Routing(c)

		// unique hostname match, any port
		got, err := routing.Resolve(context.Background(), store, "a.example", 9090)
		if err != nil {
			t.Fatal(err)
		}
		if got.TenantID != 1 || got.RootPageID != homeA.ID {
			t.Fatalf("got %+v", got)
		}

		// no hostname match falls back to the default site
		got, err = routing.Resolve(context.Background(), store, "unknown.example", 80)
		if err != nil {
			t.Fatal(err)
		}
		if got.TenantID != 2 || got.ID != fallback.ID {
			t.Fatalf("got %+v, want the default site", got)
		}

		// hostname+port disambiguates when one hostname has several ports
		homeA2 := mkPage(c, a, "0002", "home-a2")
		mkSite(c, a, "a.example", 8443, false, homeA2)
		got, err = routing.Resolve(context.Background(), store, "a.example", 8443)
		if err != nil {
			t.Fatal(err)
		}
		if got.RootPageID != homeA2.ID {
			t.Fatalf("got %+v, want the 8443 site", got)
		}

		// hostname is globally unique, not per tenant: tenant 2 cannot reuse tenant 1's hostname
		if _, err := c.Site.Create().SetHostname("a.example").SetPort(80).SetRootPage(homeB).Save(b); err == nil {
			t.Fatal("duplicate hostname across tenants should be rejected")
		}
	})
}

func TestSiteMiddleware(t *testing.T) {
	eachBackend(t, func(t *testing.T, url func(string) string) {
		c := newClient(t, url(""))
		a := tenant.With(context.Background(), 1)
		home := mkPage(c, a, "0001", "home")
		mkSite(c, a, "mw.example", 80, false, home)

		var gotTenant int
		var gotSite routing.Site
		mw := routing.Middleware(site.Routing(c))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotTenant, _ = tenant.From(r.Context())
			gotSite, _ = routing.FromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, httptest.NewRequest("GET", "http://mw.example/", nil))
		if rr.Code != http.StatusOK || gotTenant != 1 || gotSite.RootPageID != home.ID {
			t.Fatalf("code=%d tenant=%d site=%+v", rr.Code, gotTenant, gotSite)
		}

		rr2 := httptest.NewRecorder()
		mw.ServeHTTP(rr2, httptest.NewRequest("GET", "http://no-such-site.example/", nil))
		if rr2.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", rr2.Code)
		}
	})
}
