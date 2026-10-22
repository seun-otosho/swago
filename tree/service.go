package tree

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrConflict     = errors.New("tree: concurrent write conflict")
	ErrSlugTaken    = errors.New("tree: a sibling already uses this slug")
	ErrMoveIntoSelf = errors.New("tree: cannot move a page under itself")
	ErrNotFound     = errors.New("tree: page not found")
)

// Node is the tree's view of a page.
type Node struct {
	ID       int
	Path     string
	Depth    int
	NumChild int
	Slug     string
	URLPath  string
}

// NewNode is a page to insert; the tree fills in Path, Depth and URLPath.
type NewNode struct {
	Path, URLPath     string
	Depth             int
	Title, Slug, Type string
	Data              map[string]any
}

// Input is what a caller supplies when adding a page.
type Input struct {
	Title, Slug, Type string
	Data              map[string]any
}

// Store is the storage a project provides over its generated Ent client. It runs inside
// one transaction, scoped to the tenant in the context. Implementations return
// ErrNotFound from Get* and ErrConflict from Create when a unique index is violated.
type Store interface {
	Get(ctx context.Context, id int) (Node, error)
	GetForUpdate(ctx context.Context, id int) (Node, error) // locks the row until the transaction ends, where the database supports it
	GetByPath(ctx context.Context, path string) (Node, error)
	LastChildPath(ctx context.Context, parentPath string) (path string, ok bool, err error)
	URLPathExists(ctx context.Context, urlPath string) (bool, error)
	Create(ctx context.Context, n NewNode) (Node, error)
	AddNumChild(ctx context.Context, id, delta int) error
	Subtree(ctx context.Context, path string) ([]Node, error) // path and all descendants
	Relocate(ctx context.Context, id int, path string, depth int, urlPath string) error
	DeleteSubtree(ctx context.Context, path string) error
}

// Tx is a Store bound to one database transaction.
type Tx interface {
	Store
	Commit() error
	Rollback() error
}

// Tree edits the page tree. Every method runs in one transaction and retries when two
// writers pick the same path for a new sibling (the unique (tenant_id, path) index decides).
// Children are serialized by locking the parent row; root inserts only have the retry.
// ponytail: concurrent AddRoot relies on retry alone, add a tenant-level lock if roots get busy.
type Tree struct {
	Begin func(context.Context) (Tx, error)
}

const attempts = 5

func (t Tree) run(ctx context.Context, fn func(Store) error) (err error) {
	for i := 0; i < attempts; i++ {
		var tx Tx
		if tx, err = t.Begin(ctx); err != nil {
			return err
		}
		if err = fn(tx); err == nil {
			if err = tx.Commit(); err == nil {
				return nil
			}
		} else {
			tx.Rollback()
		}
		if !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return err
}

func nextChildPath(ctx context.Context, s Store, parentPath string) (string, error) {
	last, ok, err := s.LastChildPath(ctx, parentPath)
	if err != nil {
		return "", err
	}
	if !ok {
		return FirstChild(parentPath), nil
	}
	return NextSibling(last)
}

func create(ctx context.Context, s Store, parent *Node, in Input) (Node, error) {
	parentPath, parentURL, depth := "", "/", 1
	if parent != nil {
		parentPath, parentURL, depth = parent.Path, parent.URLPath, parent.Depth+1
	}
	urlPath := parentURL + in.Slug + "/"
	if taken, err := s.URLPathExists(ctx, urlPath); err != nil {
		return Node{}, err
	} else if taken {
		return Node{}, ErrSlugTaken
	}
	path, err := nextChildPath(ctx, s, parentPath)
	if err != nil {
		return Node{}, err
	}
	n, err := s.Create(ctx, NewNode{Path: path, Depth: depth, URLPath: urlPath,
		Title: in.Title, Slug: in.Slug, Type: in.Type, Data: in.Data})
	if err != nil {
		return Node{}, err
	}
	if parent != nil {
		if err := s.AddNumChild(ctx, parent.ID, 1); err != nil {
			return Node{}, err
		}
	}
	return n, nil
}

// AddRoot adds a top-level page.
func (t Tree) AddRoot(ctx context.Context, in Input) (n Node, err error) {
	err = t.run(ctx, func(s Store) (e error) { n, e = create(ctx, s, nil, in); return })
	return
}

// AddChild adds a page as the last child of parentID.
func (t Tree) AddChild(ctx context.Context, parentID int, in Input) (n Node, err error) {
	err = t.run(ctx, func(s Store) error {
		parent, e := s.GetForUpdate(ctx, parentID) // serializes sibling inserts under one parent
		if e != nil {
			return e
		}
		n, e = create(ctx, s, &parent, in)
		return e
	})
	return
}

// Move re-parents id and its whole subtree as the last child of newParentID, rewriting
// every path, depth and url_path below it.
func (t Tree) Move(ctx context.Context, id, newParentID int) error {
	return t.run(ctx, func(s Store) error {
		node, err := s.Get(ctx, id)
		if err != nil {
			return err
		}
		dest, err := s.GetForUpdate(ctx, newParentID)
		if err != nil {
			return err
		}
		if strings.HasPrefix(dest.Path, node.Path) {
			return ErrMoveIntoSelf
		}
		oldParentPath := Parent(node.Path)
		if oldParentPath == dest.Path {
			return nil
		}
		newURL := dest.URLPath + node.Slug + "/"
		if taken, err := s.URLPathExists(ctx, newURL); err != nil {
			return err
		} else if taken {
			return ErrSlugTaken
		}
		newPath, err := nextChildPath(ctx, s, dest.Path)
		if err != nil {
			return err
		}
		subtree, err := s.Subtree(ctx, node.Path)
		if err != nil {
			return err
		}
		for _, n := range subtree {
			p := Rebase(n.Path, node.Path, newPath)
			u := newURL + strings.TrimPrefix(n.URLPath, node.URLPath)
			if err := s.Relocate(ctx, n.ID, p, Depth(p), u); err != nil {
				return err
			}
		}
		if oldParentPath != "" {
			old, err := s.GetByPath(ctx, oldParentPath)
			if err != nil {
				return err
			}
			if err := s.AddNumChild(ctx, old.ID, -1); err != nil {
				return err
			}
		}
		return s.AddNumChild(ctx, dest.ID, 1)
	})
}

// Delete removes id and everything below it.
func (t Tree) Delete(ctx context.Context, id int) error {
	return t.run(ctx, func(s Store) error {
		node, err := s.Get(ctx, id)
		if err != nil {
			return err
		}
		if err := s.DeleteSubtree(ctx, node.Path); err != nil {
			return err
		}
		if p := Parent(node.Path); p != "" {
			parent, err := s.GetByPath(ctx, p)
			if err != nil {
				return err
			}
			return s.AddNumChild(ctx, parent.ID, -1)
		}
		return nil
	})
}
