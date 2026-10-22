package site_test

import (
	"context"
	"entgo.io/ent/dialect"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/seun-otosho/swago/examples/site"
	"github.com/seun-otosho/swago/tenant"
	"github.com/seun-otosho/swago/tree"
)

func TestTreeOperations(t *testing.T) {
	eachBackend(t, func(t *testing.T, url func(string) string) {
		c := newClient(t, url(""))
		ctx := tenant.With(context.Background(), 1)
		tr := site.Tree(c, dialectOf(t))
		in := func(slug string) tree.Input { return tree.Input{Title: slug, Slug: slug, Type: "article"} }

		home, err := tr.AddRoot(ctx, in("home"))
		must(t, err)
		blog, err := tr.AddChild(ctx, home.ID, in("blog"))
		must(t, err)
		about, err := tr.AddChild(ctx, home.ID, in("about"))
		must(t, err)
		post, err := tr.AddChild(ctx, blog.ID, in("hello"))
		must(t, err)

		eq(t, home.Path, "0001")
		eq(t, blog.Path, "00010001")
		eq(t, about.Path, "00010002")
		eq(t, post.Path, "000100010001")
		eq(t, post.URLPath, "/home/blog/hello/")
		eq(t, c.Page.GetX(ctx, home.ID).Numchild, 2)

		if _, err := tr.AddChild(ctx, home.ID, in("blog")); !errors.Is(err, tree.ErrSlugTaken) {
			t.Fatalf("duplicate slug: %v", err)
		}

		// move blog (with its post) under about
		must(t, tr.Move(ctx, blog.ID, about.ID))
		moved := c.Page.GetX(ctx, post.ID)
		eq(t, moved.Path, "000100020001"+"0001")
		eq(t, moved.URLPath, "/home/about/blog/hello/")
		eq(t, moved.Depth, 4)
		eq(t, c.Page.GetX(ctx, home.ID).Numchild, 1)
		eq(t, c.Page.GetX(ctx, about.ID).Numchild, 1)

		if err := tr.Move(ctx, about.ID, post.ID); !errors.Is(err, tree.ErrMoveIntoSelf) {
			t.Fatalf("move into own subtree: %v", err)
		}

		// delete a subtree
		must(t, tr.Delete(ctx, about.ID))
		eq(t, c.Page.Query().CountX(ctx), 1)
		eq(t, c.Page.GetX(ctx, home.ID).Numchild, 0)

		// another tenant has its own tree with the same paths
		other, err := tr.AddRoot(tenant.With(context.Background(), 2), in("home"))
		must(t, err)
		eq(t, other.Path, "0001")
	})
}

func TestConcurrentSiblings(t *testing.T) {
	eachBackend(t, func(t *testing.T, url func(string) string) {
		c := newClient(t, url(""))
		ctx := tenant.With(context.Background(), 1)
		tr := site.Tree(c, dialectOf(t))
		home, err := tr.AddRoot(ctx, tree.Input{Title: "home", Slug: "home", Type: "article"})
		must(t, err)

		const n = 8
		var wg sync.WaitGroup
		errs := make(chan error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				slug := string(rune('a' + i))
				_, err := tr.AddChild(ctx, home.ID, tree.Input{Title: slug, Slug: slug, Type: "article"})
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			must(t, err)
		}
		eq(t, c.Page.GetX(ctx, home.ID).Numchild, n)
		eq(t, c.Page.Query().CountX(ctx), n+1)
	})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func eq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func dialectOf(t *testing.T) string {
	if strings.HasSuffix(t.Name(), "/postgres") {
		return dialect.Postgres
	}
	return dialect.SQLite
}
