package tenant

import (
	"ariga.io/atlas/sql/schema"
	entschema "entgo.io/ent/dialect/sql/schema"
)

// CompositeFKs rewrites every single-column foreign key between tenant-bearing tables
// into (tenant_id, fk) -> (tenant_id, id), and adds the unique (tenant_id, id) index the
// referenced table needs, so the database itself refuses cross-tenant references. Pass it
// to every Ent migration call (Schema.Create and Schema.Diff).
//
// ponytail: optional edges with ON DELETE SET NULL would also null tenant_id; keep such
// edges NO ACTION or use Postgres 15+ SET NULL (col) before relying on them.
var CompositeFKs = entschema.WithDiffHook(func(next entschema.Differ) entschema.Differ {
	return entschema.DiffFunc(func(current, desired *schema.Schema) ([]schema.Change, error) {
		for _, t := range desired.Tables {
			tid, ok := t.Column("tenant_id")
			if !ok {
				continue
			}
			for _, fk := range t.ForeignKeys {
				ref := fk.RefTable
				rid, ok := ref.Column("tenant_id")
				if !ok || len(fk.Columns) != 1 {
					continue
				}
				id, _ := ref.Column("id")
				if !hasUnique(ref, "tenant_id", "id") {
					ref.AddIndexes(schema.NewUniqueIndex(ref.Name+"_tenant_id_id").AddColumns(rid, id))
				}
				fk.Columns = []*schema.Column{tid, fk.Columns[0]}
				fk.RefColumns = []*schema.Column{rid, id}
			}
		}
		return next.Diff(current, desired)
	})
})

func hasUnique(t *schema.Table, cols ...string) bool {
	for _, ix := range t.Indexes {
		if !ix.Unique || len(ix.Parts) != len(cols) {
			continue
		}
		match := true
		for i, p := range ix.Parts {
			match = match && p.C != nil && p.C.Name == cols[i]
		}
		if match {
			return true
		}
	}
	return false
}
