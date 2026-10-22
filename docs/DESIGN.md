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
