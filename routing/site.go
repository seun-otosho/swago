// Package routing resolves an incoming request's hostname to a site and, through it, a
// tenant, the way Wagtail's Site model does. Matching order: a single site with this
// hostname regardless of port, else hostname plus port, else the default site, else no
// match.
//
// Core cannot import a project's generated Ent types (the same boundary package tree
// works around), so resolution runs against a small Store interface a project implements
// over its client.
package routing

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"

	"github.com/seun-otosho/swago/tenant"
)

// ErrNoSite is returned when no site matches the request and there is no default site.
var ErrNoSite = errors.New("routing: no site matches this request and there is no default site")

// Site is routing's view of a site: enough to put a tenant in context and know where to
// start serving pages from.
type Site struct {
	ID         int
	TenantID   int
	Hostname   string
	Port       int
	SiteName   string
	IsDefault  bool
	RootPageID int
	// RootURLPath is the root page's url_path (e.g. "/home/"); strip it from a page's own
	// url_path to get the path this site serves that page at.
	RootURLPath string
}

// Store is the lookup a project provides over its generated Ent client. Resolve always
// calls it with tenant.AllowWithoutTenant already applied: site resolution runs before the
// tenant is known, so it is the one place allowed to see every tenant's sites.
type Store interface {
	// ByHostname returns every site with this hostname, any port.
	ByHostname(ctx context.Context, hostname string) ([]Site, error)
	ByHostnamePort(ctx context.Context, hostname string, port int) (Site, bool, error)
	// Default returns the fallback site, if one is marked is_default_site. More than one
	// is a schema-level gap (see schema.Site); implementations should return a
	// deterministic choice (e.g. the lowest id) rather than an arbitrary row.
	Default(ctx context.Context) (Site, bool, error)
}

// Resolve matches hostname and port to a site, in Wagtail's order.
func Resolve(ctx context.Context, store Store, hostname string, port int) (Site, error) {
	ctx = tenant.AllowWithoutTenant(ctx)

	sites, err := store.ByHostname(ctx, hostname)
	if err != nil {
		return Site{}, err
	}
	if len(sites) == 1 {
		return sites[0], nil
	}

	if s, ok, err := store.ByHostnamePort(ctx, hostname, port); err != nil {
		return Site{}, err
	} else if ok {
		return s, nil
	}

	if s, ok, err := store.Default(ctx); err != nil {
		return Site{}, err
	} else if ok {
		return s, nil
	}

	return Site{}, ErrNoSite
}

// HostPort splits an HTTP request's Host header into the hostname and port Resolve
// expects: no scheme, defaulting to 443 over TLS and 80 otherwise when the header carries
// no explicit port.
func HostPort(r *http.Request) (hostname string, port int) {
	host, portStr, err := net.SplitHostPort(r.Host)
	if err != nil {
		host, portStr = r.Host, ""
	}
	port = 80
	if r.TLS != nil {
		port = 443
	}
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}
	return host, port
}

type ctxKey int

const siteKey ctxKey = 0

// WithSite returns ctx carrying site, retrievable later with FromContext.
func WithSite(ctx context.Context, s Site) context.Context { return context.WithValue(ctx, siteKey, s) }

// FromContext returns the site Middleware resolved for this request, if any.
func FromContext(ctx context.Context) (Site, bool) {
	s, ok := ctx.Value(siteKey).(Site)
	return s, ok
}

// Middleware resolves the site from the request's Host header, puts the tenant and the
// site into context, and serves next. A request matching no site gets a 404.
func Middleware(store Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, port := HostPort(r)
			s, err := Resolve(r.Context(), store, host, port)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			ctx := WithSite(tenant.With(r.Context(), s.TenantID), s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
