package site

import (
	"context"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"github.com/seun-otosho/swago/examples/site/ent"
	"github.com/seun-otosho/swago/examples/site/ent/page"
	"github.com/seun-otosho/swago/tree"
)

// Tree returns the page-tree editor backed by c. d is the database dialect
// (dialect.SQLite or dialect.Postgres): row locks are skipped on SQLite, whose
// immediate transactions already serialize writers.
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
