// Package tenant scopes every Ent query and mutation to the tenant in the context.
// It fails closed: with no tenant in the context, operations are refused.
package tenant

import (
	"context"
	"errors"
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type ctxKey int

const (
	tenantKey ctxKey = iota
	allowKey
)

// ErrNoTenant is returned when an operation runs without a tenant in its context.
var ErrNoTenant = errors.New("tenant: no tenant in context")

// With returns ctx scoped to tenant id.
func With(ctx context.Context, id int) context.Context {
	return context.WithValue(ctx, tenantKey, id)
}

// AllowWithoutTenant is the greppable escape hatch for system jobs and superusers.
func AllowWithoutTenant(ctx context.Context) context.Context {
	return context.WithValue(ctx, allowKey, true)
}

// From returns the tenant id in ctx, if any.
func From(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(tenantKey).(int)
	return id, ok
}

func scope(ctx context.Context) (id int, all bool, err error) {
	if id, ok := From(ctx); ok {
		return id, false, nil
	}
	if a, _ := ctx.Value(allowKey).(bool); a {
		return 0, true, nil
	}
	return 0, false, ErrNoTenant
}

// Wherer is what a query or mutation needs to expose to be scoped.
type Wherer interface{ WhereP(...func(*sql.Selector)) }

// QueryAdapter wraps a generated query so it can be scoped. Ent only exposes WhereP on
// queries through each project's generated ent/intercept package, so the project sets
// this once at startup: tenant.QueryAdapter = func(q ent.Query) (tenant.Wherer, error) {
// return intercept.NewQuery(q) }. Until it is set, every query is refused.
var QueryAdapter func(ent.Query) (Wherer, error)

// Mixin adds tenant_id plus the interceptor and hook that enforce it.
type Mixin struct{ mixin.Schema }

func (Mixin) Fields() []ent.Field {
	return []ent.Field{field.Int("tenant_id").Immutable()}
}

func (Mixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{ent.TraverseFunc(func(ctx context.Context, q ent.Query) error {
		id, all, err := scope(ctx)
		if err != nil || all {
			return err
		}
		if QueryAdapter == nil {
			return errors.New("tenant: QueryAdapter not set")
		}
		w, err := QueryAdapter(q)
		if err != nil {
			return err
		}
		w.WhereP(sql.FieldEQ("tenant_id", id))
		return nil
	})}
}

func (Mixin) Hooks() []ent.Hook {
	return []ent.Hook{func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			id, all, err := scope(ctx)
			if err != nil {
				return nil, err
			}
			if all {
				return next.Mutate(ctx, m)
			}
			if m.Op().Is(ent.OpCreate) {
				if v, ok := m.Field("tenant_id"); ok && v != id {
					return nil, fmt.Errorf("tenant: create for tenant %v in context of %d", v, id)
				}
				if err := m.SetField("tenant_id", id); err != nil {
					return nil, err
				}
				return next.Mutate(ctx, m)
			}
			w, ok := m.(Wherer)
			if !ok {
				return nil, fmt.Errorf("tenant: mutation %T cannot be scoped", m)
			}
			w.WhereP(sql.FieldEQ("tenant_id", id))
			return next.Mutate(ctx, m)
		})
	}}
}
