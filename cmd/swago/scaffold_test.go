package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestScaffoldBuildsAndMigrates scaffolds a project against this checkout (via a replace
// directive, since swago is not published), then proves the output is not just
// well-formed but actually compiles, vets, migrates, serves and renders a real page
// resolved through site routing: go generate, go build, go vet, go run . migrate, a
// throwaway seed through the generated client, go run . serve, and an HTTP request.
func TestScaffoldBuildsAndMigrates(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "demo")
	dbURL := "sqlite://file:" + filepath.Join(dir, "demo.db") + "?mode=rwc"

	if err := scaffold(dir, "demo", repoRoot); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	run := func(env []string, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if env != nil {
			cmd.Env = append(os.Environ(), env...)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}

	run(nil, "go", "generate", "./ent/...")
	run(nil, "go", "build", "./...")
	run(nil, "go", "vet", "./...")
	run([]string{"DATABASE_URL=" + dbURL}, "go", "run", ".", "migrate")

	// Seed a tenant, a two-level page tree and a site, through a throwaway test file using
	// the scaffold's own generated client -- the same code path a real project uses.
	seed := filepath.Join(dir, "seed_test.go")
	if err := os.WriteFile(seed, []byte(seedSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	run([]string{"DATABASE_URL=" + dbURL}, "go", "test", "-run", "TestSeed", ".")
	if err := os.Remove(seed); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(dir, "demo-bin")
	run(nil, "go", "build", "-o", bin, ".")

	serve := exec.Command(bin, "serve", "--addr", ":18099")
	serve.Dir = dir
	serve.Env = append(os.Environ(), "DATABASE_URL="+dbURL)
	var serveOut strings.Builder
	serve.Stdout, serve.Stderr = &serveOut, &serveOut
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serve.Process.Kill(); serve.Wait() })

	get := func(path string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", "http://127.0.0.1:18099"+path, nil)
		req.Host = "localhost:18099"
		var resp *http.Response
		var err error
		for range 50 {
			resp, err = http.DefaultClient.Do(req)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("GET %s: %v\nserver output:\n%s", path, err, serveOut.String())
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	if code, body := get("/"); code != 200 || !strings.Contains(body, "Welcome Home") {
		t.Fatalf("GET / = %d %q", code, body)
	}
	if code, body := get("/blog/"); code != 200 || !strings.Contains(body, "The Blog") {
		t.Fatalf("GET /blog/ = %d %q", code, body)
	}
	if code, _ := get("/nope/"); code != 404 {
		t.Fatalf("GET /nope/ = %d, want 404", code)
	}
}

const seedSrc = `package main

import (
	"context"
	"testing"

	"github.com/seun-otosho/swago/tenant"
)

func TestSeed(t *testing.T) {
	c, err := open(context.Background(), "sqlite://file:demo.db?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := tenant.With(context.Background(), 1)
	home := c.Page.Create().SetPath("0001").SetDepth(1).SetTitle("Welcome Home").SetSlug("home").
		SetURLPath("/home/").SetType("article").SaveX(ctx)
	c.Page.Create().SetPath("00010001").SetDepth(2).SetTitle("The Blog").SetSlug("blog").
		SetURLPath("/home/blog/").SetType("article").SaveX(ctx)
	c.Site.Create().SetHostname("localhost").SetPort(18099).SetRootPage(home).SaveX(ctx)
}
`
