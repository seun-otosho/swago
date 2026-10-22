# swago: design notes

A Go CMS inspired by Wagtail. Date: 2026-10-22. Status: planning, no code yet.

## Decision: Go-native rethink, not a port

Full parity with Wagtail and a Go-native rethink pull in opposite directions. We take the rethink and treat parity as a checklist to tick off later. We do **not** promise API or database compatibility with Wagtail. That compatibility is where most of the cost of full parity goes.

| | Full parity (port) | Go-native rethink (chosen) |
|---|---|---|
| Schema definition | Runtime, DB-driven (Django models and migrations). Go has no metaclasses, so it would need reflection or codegen to imitate them. | Page types are Go structs registered in code. Codegen produces migrations and admin forms. Checked at compile time. |
| Extensibility | Relies on Python monkey-patching, hooks and signals. Third-party packages can't be ported as they are. | Go modules, explicit interfaces and middleware. Extensions are compiled in. |
| Page templates | Django templates, `{% include_block %}`. | `html/template` or templ, with typed blocks. |
| StreamField | JSON blob, runtime block definitions. | Same JSON on disk, but blocks are Go types with a tagged union for (de)serialization. |
| Admin UI | Re-create Draftail, page chooser, explorer and so on. The largest part of the work. | Embedded SPA or htmx, with server-rendered forms generated from struct tags. |
| Data compatibility | Can read existing Wagtail databases. | Clean schema, plus an optional one-way importer. |
| Cost | Multi-year, and Wagtail keeps moving. | Months for a usable core. We choose what to leave out. |
| Deployment | Single binary. | Single binary. |

Wagtail features to reach eventually (the parity checklist): page tree, StreamField, admin UI, images and renditions, documents, search, workflows, permissions, i18n, forms, snippets.

## Datastore

- **SQLite is the default**, for dev and zero-ops deployments.
- Also support everything Django/Wagtail supports (Postgres, MySQL/MariaDB), and others later.
- Query layer: [`bun`](https://bun.uptrace.dev) or `sqlx` with a small dialect layer. `sqlc` is nicer but needs one query set per dialect. Open decision.
- Page tree: materialized paths (like treebeard), because they work on every database.
- Block JSON is stored as TEXT/JSON, again so it works everywhere. No Postgres-only features (ltree, JSONB operators) in core.

## Build order

1. Page tree, plus typed page models registered in code.
2. Blocks (the StreamField equivalent) and revisions with draft/publish.
3. Headless JSON API and server-rendered templates.
4. Auth, permissions and groups.
5. Admin UI.
6. Images and renditions, documents, search, i18n, workflows.

## Open questions

- Module path and Go version. Not yet chosen.
- Query layer: `bun` vs `sqlx`.
- Admin UI: htmx (server-rendered) vs embedded SPA.
- Template engine: `html/template` vs templ.

## Review: gaps and open questions

Review dated 2026-10-22, written against the planning-stage doc above. The core call (rethink over port) is sound, and the comparison table is the strongest part.

1. **Two open questions are already answered by the project's own goal.** The project is "Go, templ, htmx", yet the open questions list htmx vs SPA and `html/template` vs templ. Recommend closing both: templ for templates, htmx for the admin, with small JS islands where needed (see point 6).
2. **How page fields are stored is not decided, and it is the biggest schema decision.** Wagtail uses multi-table inheritance (a base page table plus one table per page type). The alternative is one pages table with typed fields in a JSON column. This choice drives codegen, migrations, revisions, querying by field, and the importer. It belongs in track 2 before any code.
3. **"Codegen produces migrations" hides real work.** When a struct changes, something has to diff the old and new schema, handle renames, and generate migrations for three dialects. No tool is named (Atlas, goose plus a custom differ, bun migrations). Also unclear: how core migrations and user page-type migrations are ordered and versioned.
4. **Materialized path details are missing.** Path alphabet and step width (which caps children per node), how `depth` and `numchild` are kept in sync, the cost of moving a subtree, and concurrent sibling inserts (a known race in treebeard, and SQLite has a single writer).
5. **Sites and routing are absent.** Wagtail's Site model (hostname to root page), slug uniqueness among siblings, cached URL paths, and serving a request by walking the tree all need a home, probably in step 1.
6. **Rich text and the StreamField editor are not addressed.** Draftail has no htmx equivalent. A rich text editor needs JS (Tiptap, ProseMirror or Lexical), and a nested, reorderable StreamField editor is the hardest UI in Wagtail. This is the main risk to the htmx choice and should be what the prototype tests first.
7. **Single binary vs SQLite driver.** `mattn/go-sqlite3` needs cgo, which complicates cross-compiling a single binary. `modernc.org/sqlite` is pure Go. Same question later for image renditions (libvips via cgo vs pure-Go imaging). Worth stating as a principle: pure Go in core unless there is a strong reason.
8. **Multi-dialect from day one multiplies cost.** Every query, migration and test runs against three databases. `bun` supports all three and fits the "works everywhere" rule; `sqlc` would need one query set per dialect. Recommend `bun` and a CI matrix from the start, or ship SQLite plus Postgres first and add MySQL later.
9. **Search conflicts with "no Postgres-only features".** Full text search differs on each database (SQLite FTS5, Postgres tsvector, MySQL FULLTEXT). Needs a backend interface like Wagtail's, with a basic fallback.
10. **Features placed late shape early schema.** i18n (Wagtail adds `locale` and `translation_key` to every page), permissions (page permissions are granted on tree paths), and workflows and scheduled publishing (which hang off revisions) all affect the tables built in steps 1 and 2. Reserve the columns and hooks early even if the features come later.
11. **Build order leaves no way to edit content until step 5.** A headless API at step 3 also needs auth for previews and writes, but auth comes at step 4. Suggest a thin vertical slice instead: tree, one page type, draft/publish, a minimal login and a bare admin edit form, then widen each layer.
12. **Not mentioned at all:** preview, page locking, redirects, audit log, sitemaps, caching, the extension/hook registry design (the doc says "explicit interfaces" but not which ones), file storage backends (local vs S3), and a testing strategy.

## Relation to the three tracks

All three tracks stand; this doc supports each one and gives each a concrete starting point.

**Track 1: Map Wagtail features to Go.** The comparison table and parity checklist are this track's seed. The track should turn the checklist into a per-feature table (Wagtail concept, Go design, build step, in or out of the first release) and close the open questions above: query layer, migration tool, SQLite driver, and the hook/extension model. Review points 3, 7, 8, 9 and 12 land here.

**Track 2: Build the page tree model.** This is step 1 of the build order and the critical path. The doc gives the direction (structs registered in code, materialized paths) but leaves the decisions that block code: page field storage (point 2), path encoding and concurrency (point 4), sites and routing (point 5), and reserved columns for locale, revisions and permissions (point 10). It depends on track 1 picking the query layer, so settle that first.

**Track 3: Prototype the admin with htmx.** The doc puts the admin at step 5 and still lists htmx as open, so an early prototype is how that question gets closed. Run it as a spike against a stubbed page tree, not a finished one, and aim it at the hardest parts: the page explorer, a nested StreamField editor, and rich text (point 6). If those work with htmx plus small JS islands, the choice is settled.

## Suggested next steps

1. Update this doc to close templ and htmx as decided, and record the pure-Go principle.
2. Start track 1 and track 2 together; track 1's first job is the query layer and migration tool, since track 2 needs them.
3. Start track 3 once track 2 has a sketch of the page-type registration API, so the prototype's forms match the real struct shape.
