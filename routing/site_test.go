package routing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seun-otosho/swago/tenant"
)

// fakeStore is an in-memory Store for testing Resolve and Middleware without a database.
type fakeStore struct {
	sites []Site
	dflt  int // index into sites, -1 for none
	calls []string
}

func (s *fakeStore) ByHostname(_ context.Context, hostname string) ([]Site, error) {
	s.calls = append(s.calls, "ByHostname")
	var out []Site
	for _, site := range s.sites {
		if site.Hostname == hostname {
			out = append(out, site)
		}
	}
	return out, nil
}

func (s *fakeStore) ByHostnamePort(_ context.Context, hostname string, port int) (Site, bool, error) {
	s.calls = append(s.calls, "ByHostnamePort")
	for _, site := range s.sites {
		if site.Hostname == hostname && site.Port == port {
			return site, true, nil
		}
	}
	return Site{}, false, nil
}

func (s *fakeStore) Default(_ context.Context) (Site, bool, error) {
	s.calls = append(s.calls, "Default")
	if s.dflt < 0 {
		return Site{}, false, nil
	}
	return s.sites[s.dflt], true, nil
}

func TestResolveSingleHostnameMatch(t *testing.T) {
	store := &fakeStore{dflt: -1, sites: []Site{{ID: 1, Hostname: "example.com", Port: 80}}}
	got, err := Resolve(context.Background(), store, "example.com", 8080)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 {
		t.Fatalf("got site %d, want 1", got.ID)
	}
	// a single hostname match short-circuits before the port-specific lookup
	for _, c := range store.calls {
		if c == "ByHostnamePort" || c == "Default" {
			t.Fatalf("unexpected call %v after a single hostname match: %v", c, store.calls)
		}
	}
}

func TestResolveHostnamePortMatch(t *testing.T) {
	store := &fakeStore{dflt: -1, sites: []Site{
		{ID: 1, Hostname: "example.com", Port: 80},
		{ID: 2, Hostname: "example.com", Port: 8080},
	}}
	got, err := Resolve(context.Background(), store, "example.com", 8080)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 {
		t.Fatalf("got site %d, want 2", got.ID)
	}
}

func TestResolveFallsBackToDefault(t *testing.T) {
	store := &fakeStore{dflt: 0, sites: []Site{{ID: 9, Hostname: "fallback.example", Port: 80}}}
	got, err := Resolve(context.Background(), store, "unknown.example", 80)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 9 {
		t.Fatalf("got site %d, want 9", got.ID)
	}
}

func TestResolveNoMatch(t *testing.T) {
	store := &fakeStore{dflt: -1}
	if _, err := Resolve(context.Background(), store, "unknown.example", 80); !errors.Is(err, ErrNoSite) {
		t.Fatalf("got %v, want ErrNoSite", err)
	}
}

func TestHostPort(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.com:9090/", nil)
	if h, p := HostPort(r); h != "example.com" || p != 9090 {
		t.Fatalf("got %q:%d", h, p)
	}
	r2 := httptest.NewRequest("GET", "http://example.com/", nil)
	if h, p := HostPort(r2); h != "example.com" || p != 80 {
		t.Fatalf("got %q:%d, want example.com:80", h, p)
	}
}

func TestMiddleware(t *testing.T) {
	store := &fakeStore{dflt: -1, sites: []Site{{ID: 1, TenantID: 7, Hostname: "example.com", Port: 80, RootPageID: 3}}}
	var gotTenant int
	var gotSite Site
	mw := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant, _ = tenant.From(r.Context())
		gotSite, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, httptest.NewRequest("GET", "http://example.com/", nil))
	if rr.Code != http.StatusOK || gotTenant != 7 || gotSite.ID != 1 {
		t.Fatalf("code=%d tenant=%d site=%+v", rr.Code, gotTenant, gotSite)
	}

	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, httptest.NewRequest("GET", "http://unmatched.example/", nil))
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rr2.Code)
	}
}
