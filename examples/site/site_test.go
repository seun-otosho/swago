package site_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect/sql/schema"

	swagodb "github.com/seun-otosho/swago/db"
	"github.com/seun-otosho/swago/examples/site"
	"github.com/seun-otosho/swago/examples/site/ent"
	emigrate "github.com/seun-otosho/swago/examples/site/ent/migrate"
	"github.com/seun-otosho/swago/examples/site/ent/page"
	"github.com/seun-otosho/swago/tenant"
)

// SWAGO_TEST_POSTGRES is a server URL ending in "/", e.g. postgres://u@127.0.0.1:54329/.
// It needs two empty databases named "swago_test" and "swago_dev". Without it only SQLite runs.
var pgBase = os.Getenv("SWAGO_TEST_POSTGRES")

// dbURL returns a fresh, empty database URL for each backend.
func backends(t *testing.T) map[string]func(name string) string {
	m := map[string]func(string) string{
		"sqlite": func(name string) string {
			return "sqlite://file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + name + "?mode=memory&cache=shared"
		},
	}
	if pgBase != "" {
		m["postgres"] = func(name string) string {
			url := pgBase + map[string]string{"": "swago_test", "dev": "swago_dev"}[name]
			db, err := sql.Open("pgx", url)
			if err == nil {
				_, err = db.Exec("drop schema public cascade; create schema public")
				db.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			return url
		}
	}
	return m
}

func eachBackend(t *testing.T, fn func(t *testing.T, url func(name string) string)) {
	for name, url := range backends(t) {
		t.Run(name, func(t *testing.T) { fn(t, url) })
	}
}

func newClient(t *testing.T, url string) *ent.Client {
	c, err := site.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func mkPage(c *ent.Client, ctx context.Context, path, slug string) *ent.Page {
	return c.Page.Create().SetPath(path).SetDepth(len(path) / 4).SetTitle(slug).SetSlug(slug).
		SetURLPath("/" + slug + "/").SetType("article").SaveX(ctx)
}

func TestTenantIsolation(t *testing.T) {
	eachBackend(t, func(t *testing.T, url func(string) string) {
		c := newClient(t, url(""))
		bg := context.Background()
		a, b := tenant.With(bg, 1), tenant.With(bg, 2)

		pa, pb := mkPage(c, a, "0001", "a"), mkPage(c, b, "0001", "b") // same path, different tenants
		if pa.TenantID != 1 || pb.TenantID != 2 {
			t.Fatal("tenant not stamped from context")
		}
		c.Article.Create().SetAuthor("x").SetPage(pa).SaveX(a)

		if n := c.Page.Query().CountX(a); n != 1 {
			t.Fatalf("list leaked: %d", n)
		}
		if _, err := c.Page.Get(a, pb.ID); err == nil {
			t.Fatal("get leaked")
		}
		if n, _ := c.Page.Update().Where(page.ID(pb.ID)).SetTitle("hacked").Save(a); n != 0 {
			t.Fatal("update leaked")
		}
		if _, err := c.Page.UpdateOneID(pb.ID).SetTitle("hacked").Save(a); err == nil {
			t.Fatal("update-one leaked")
		}
		if err := c.Page.DeleteOneID(pb.ID).Exec(a); err == nil {
			t.Fatal("delete leaked")
		}
		if got := c.Page.GetX(b, pb.ID); got.Title != "b" {
			t.Fatal("tenant 2 row changed")
		}
		if c.Article.Query().CountX(b) != 0 {
			t.Fatal("article leaked")
		}
		if _, err := c.Page.Create().SetTenantID(2).SetPath("0002").SetDepth(1).SetTitle("z").SetSlug("z").
			SetURLPath("/z/").SetType("article").Save(a); err == nil {
			t.Fatal("create for another tenant allowed")
		}
		if _, err := c.Page.Query().Count(bg); !errors.Is(err, tenant.ErrNoTenant) {
			t.Fatalf("not fail-closed: %v", err)
		}
		if n := c.Page.Query().CountX(tenant.AllowWithoutTenant(bg)); n != 2 {
			t.Fatalf("escape hatch sees %d rows, want 2", n)
		}
		// the database itself refuses a unique-path clash within one tenant
		if _, err := c.Page.Create().SetPath("0001").SetDepth(1).SetTitle("dup").SetSlug("dup").
			SetURLPath("/dup/").SetType("article").Save(a); err == nil {
			t.Fatal("duplicate path allowed within a tenant")
		}
	})
}

// The database must refuse a row that points at another tenant's row, even if the
// application layer is bypassed.
func TestCrossTenantReference(t *testing.T) {
	eachBackend(t, func(t *testing.T, url func(string) string) {
		c := newClient(t, url(""))
		bg := context.Background()
		a, b := tenant.With(bg, 1), tenant.With(bg, 2)
		pa := mkPage(c, a, "0001", "a")
		if _, err := c.Article.Create().SetAuthor("evil").SetPageID(pa.ID).Save(b); err == nil {
			t.Fatal("tenant 2 attached an article to tenant 1's page")
		}
	})
}

// Self-discovering: a new entity without tenant_id fails here with no test changes.
func TestEveryTableHasTenantID(t *testing.T) {
	for _, tb := range emigrate.Tables {
		found := false
		for _, col := range tb.Columns {
			found = found || col.Name == "tenant_id"
		}
		if !found {
			t.Errorf("table %s has no tenant_id (many-to-many edges create such tables; use a through-entity)", tb.Name)
		}
	}
}

// Generate versioned SQL from an empty dev database, apply it to an empty database, then
// re-diff against the directory: the Ent schema and the migration files must agree.
func TestVersionedMigrations(t *testing.T) {
	eachBackend(t, func(t *testing.T, url func(string) string) {
		ctx := context.Background()
		dir1 := t.TempDir()
		dir, err := migrate.NewLocalDir(dir1)
		if err != nil {
			t.Fatal(err)
		}
		devDrv, err := swagodb.Open(url("dev"))
		if err != nil {
			t.Fatal(err)
		}
		dev := ent.NewClient(ent.Driver(devDrv))
		t.Cleanup(func() { dev.Close() })
		diff := func() error {
			return dev.Schema.Diff(tenant.AllowWithoutTenant(ctx), schema.WithDir(dir), schema.WithDialect(devDrv.Dialect()), tenant.CompositeFKs)
		}
		if err := diff(); err != nil {
			t.Fatal("generate:", err)
		}
		ups, _ := filepath.Glob(filepath.Join(dir1, "*.up.sql"))
		downs, _ := filepath.Glob(filepath.Join(dir1, "*.down.sql"))
		if len(ups) != 1 || len(downs) != 1 {
			t.Fatalf("want one up and one down file, got %d and %d", len(ups), len(downs))
		}
		text, _ := os.ReadFile(ups[0])
		if !strings.Contains(string(text), "tenant_id") {
			t.Fatal("migration lacks tenant_id")
		}

		targetDrv, err := swagodb.Open(url(""))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { targetDrv.Close() })
		for _, stmt := range strings.Split(string(text), ";\n") {
			if s := strings.TrimSpace(stmt); s != "" && !strings.HasPrefix(s, "--") {
				if err := targetDrv.Exec(ctx, stmt, []any{}, nil); err != nil {
					t.Fatalf("apply %q: %v", stmt, err)
				}
			}
		}
		if err := diff(); err != nil { // CI drift guard
			t.Fatal("re-diff:", err)
		}
		if again, _ := filepath.Glob(filepath.Join(dir1, "*.up.sql")); len(again) != 1 {
			t.Fatalf("schema drifted from migrations: %d files after re-diff", len(again))
		}
	})
}
