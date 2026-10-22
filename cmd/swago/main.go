// Command swago scaffolds new projects, the way `wagtail start` does for Wagtail.
//
// swago new <name> creates a Go module with a starter page/site schema, the Ent-backed
// tree.Store and routing.Store implementations a project needs (see DESIGN.md's "Install,
// scaffold and extend"), and a main.go with migrate and serve subcommands.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "new" {
		usage()
	}
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	module := fs.String("module", "", "module path for the new project (default: the name)")
	replace := fs.String("replace", "", "add a go.mod replace directive pointing the swago requirement at this local checkout, for developing against an unpublished swago")
	fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		usage()
	}
	name := fs.Arg(0)
	mod := *module
	if mod == "" {
		mod = name
	}

	if err := scaffold(name, mod, *replace); err != nil {
		fmt.Fprintln(os.Stderr, "swago new:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: swago new [--module <path>] [--replace <path-to-swago>] <name>")
	os.Exit(2)
}

type data struct {
	Name    string
	Module  string
	Replace string
}

func scaffold(dir, module, replace string) error {
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	}
	d := data{Name: filepath.Base(dir), Module: module, Replace: replace}

	files := map[string]string{
		"go.mod":             goModTmpl,
		"open.go":            openTmpl,
		"store.go":           storeTmpl,
		"routing.go":         routingTmpl,
		"main.go":            mainTmpl,
		"ent/generate.go":    entGenerateTmpl,
		"ent/schema/page.go": pageSchemaTmpl,
		"ent/schema/site.go": siteSchemaTmpl,
	}
	for path, tmpl := range files {
		if err := writeTemplate(filepath.Join(dir, path), tmpl, d); err != nil {
			return err
		}
	}

	fmt.Printf("created %s (module %s)\n", dir, module)

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("go mod tidy failed, run it yourself once network/swago access is available:\n%s\n", out)
	} else {
		fmt.Println("go mod tidy ok")
	}

	fmt.Printf("next: cd %s && go run . migrate && go run . serve\n", dir)
	return nil
}

func writeTemplate(path, tmpl string, d data) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	t, err := template.New(filepath.Base(path)).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("template %s: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := t.Execute(f, d); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

const goModTmpl = `module {{.Module}}

go 1.26.4

require (
	entgo.io/ent v0.14.6
	github.com/jackc/pgx/v5 v5.11.0
	github.com/seun-otosho/swago v0.0.1
	modernc.org/sqlite v1.61.0
)

tool entgo.io/ent/cmd/ent
{{if .Replace}}
replace github.com/seun-otosho/swago => {{.Replace}}
{{end}}`

const entGenerateTmpl = `package ent

//go:generate go tool ent generate --feature intercept,sql/versioned-migration,sql/lock ./schema
`

const pageSchemaTmpl = `package schema

import "github.com/seun-otosho/swago/schema"

// Page re-exposes the core base page in this project's generated client. Add more fields
// here, or create a separate linked entity for a page type that needs queryable columns
// (swago's own examples/site/ent/schema/article.go shows that pattern).
type Page struct{ schema.Page }
`

const siteSchemaTmpl = `package schema

import "github.com/seun-otosho/swago/schema"

// Site re-exposes the core Site in this project's generated client.
type Site struct{ schema.Site }
`

const openTmpl = `package main

import (
	"context"

	entgo "entgo.io/ent"

	swagodb "github.com/seun-otosho/swago/db"
	"github.com/seun-otosho/swago/tenant"
	"{{.Module}}/ent"
	"{{.Module}}/ent/intercept"
	_ "{{.Module}}/ent/runtime" // registers the tenant hooks and interceptors
)

func init() {
	tenant.QueryAdapter = func(q entgo.Query) (tenant.Wherer, error) { return intercept.NewQuery(q) }
}

// open connects to url and applies the schema (dev auto-migrate).
func open(ctx context.Context, url string) (*ent.Client, error) {
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
`

// storeTmpl is the tree.Store implementation every project needs, mirroring
// examples/site/store.go (see DESIGN.md's "Page tree": "each project supplies an
// Ent-backed implementation (about 100 lines; swago new should generate it)").
const storeTmpl = `package main

import (
	"context"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"github.com/seun-otosho/swago/tree"
	"{{.Module}}/ent"
	"{{.Module}}/ent/page"
)

// Tree returns the page-tree editor backed by c. d is the database dialect
// (dialect.SQLite or dialect.Postgres): row locks are skipped on SQLite, whose immediate
// transactions already serialize writers.
func Tree(c *ent.Client, d string) tree.Tree {
	return tree.Tree{Begin: func(ctx context.Context) (tree.Tx, error) {
		tx, err := c.Tx(ctx)
		if err != nil {
			return nil, err
		}
		return txStore{tx, d != dialect.SQLite}, nil
	}}
}

// txStore implements tree.Tx over an Ent transaction. Tenant scoping comes from the
// context passed to each call, through the tenant mixin.
type txStore struct {
	tx   *ent.Tx
	lock bool
}

func (s txStore) Commit() error   { return s.tx.Commit() }
func (s txStore) Rollback() error { return s.tx.Rollback() }

func node(p *ent.Page) tree.Node {
	return tree.Node{ID: p.ID, Path: p.Path, Depth: p.Depth, NumChild: p.Numchild, Slug: p.Slug, URLPath: p.URLPath}
}

func mapErr(err error) error {
	switch {
	case ent.IsNotFound(err):
		return tree.ErrNotFound
	case ent.IsConstraintError(err):
		return tree.ErrConflict
	}
	return err
}

func (s txStore) Get(ctx context.Context, id int) (tree.Node, error) {
	p, err := s.tx.Page.Get(ctx, id)
	if err != nil {
		return tree.Node{}, mapErr(err)
	}
	return node(p), nil
}

func (s txStore) GetForUpdate(ctx context.Context, id int) (tree.Node, error) {
	q := s.tx.Page.Query().Where(page.ID(id))
	if s.lock {
		q = q.ForUpdate()
	}
	p, err := q.Only(ctx)
	if err != nil {
		return tree.Node{}, mapErr(err)
	}
	return node(p), nil
}

func (s txStore) GetByPath(ctx context.Context, path string) (tree.Node, error) {
	p, err := s.tx.Page.Query().Where(page.Path(path)).Only(ctx)
	if err != nil {
		return tree.Node{}, mapErr(err)
	}
	return node(p), nil
}

func (s txStore) LastChildPath(ctx context.Context, parentPath string) (string, bool, error) {
	p, err := s.tx.Page.Query().
		Where(page.PathHasPrefix(parentPath), page.Depth(tree.Depth(parentPath)+1)).
		Order(page.ByPath(sql.OrderDesc())).First(ctx)
	if ent.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return p.Path, true, nil
}

func (s txStore) URLPathExists(ctx context.Context, urlPath string) (bool, error) {
	return s.tx.Page.Query().Where(page.URLPath(urlPath)).Exist(ctx)
}

func (s txStore) Create(ctx context.Context, n tree.NewNode) (tree.Node, error) {
	c := s.tx.Page.Create().SetPath(n.Path).SetDepth(n.Depth).SetTitle(n.Title).SetSlug(n.Slug).
		SetURLPath(n.URLPath).SetType(n.Type)
	if n.Data != nil {
		c.SetData(n.Data)
	}
	p, err := c.Save(ctx)
	if err != nil {
		return tree.Node{}, mapErr(err)
	}
	return node(p), nil
}

func (s txStore) AddNumChild(ctx context.Context, id, delta int) error {
	return s.tx.Page.UpdateOneID(id).AddNumchild(delta).Exec(ctx)
}

func (s txStore) Subtree(ctx context.Context, path string) ([]tree.Node, error) {
	ps, err := s.tx.Page.Query().Where(page.PathHasPrefix(path)).Order(page.ByPath()).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]tree.Node, len(ps))
	for i, p := range ps {
		out[i] = node(p)
	}
	return out, nil
}

func (s txStore) Relocate(ctx context.Context, id int, path string, depth int, urlPath string) error {
	return mapErr(s.tx.Page.UpdateOneID(id).SetPath(path).SetDepth(depth).SetURLPath(urlPath).Exec(ctx))
}

func (s txStore) DeleteSubtree(ctx context.Context, path string) error {
	_, err := s.tx.Page.Delete().Where(page.PathHasPrefix(path)).Exec(ctx)
	return err
}
`

// routingTmpl is the routing.Store implementation every project needs, mirroring
// examples/site/routing.go.
const routingTmpl = `package main

import (
	"context"

	"github.com/seun-otosho/swago/routing"
	"{{.Module}}/ent"
	entsite "{{.Module}}/ent/site"
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
// at most one (see schema.Site), so more than one picks a deterministic row rather than an
// arbitrary one.
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
`

const mainTmpl = `package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/seun-otosho/swago/routing"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "sqlite://file:{{.Name}}.db?mode=rwc"
	}
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "migrate":
		c, err := open(context.Background(), url)
		if err != nil {
			log.Fatal(err)
		}
		c.Close()
		fmt.Println("migrated", url)
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		addr := fs.String("addr", ":8080", "listen address")
		fs.Parse(os.Args[2:])

		c, err := open(context.Background(), url)
		if err != nil {
			log.Fatal(err)
		}
		defer c.Close()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			s, _ := routing.FromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s)
		})

		log.Println("listening on", *addr)
		log.Fatal(http.ListenAndServe(*addr, routing.Middleware(Routing(c))(mux)))
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: {{.Name}} <migrate|serve>")
	os.Exit(2)
}
`
