# Swago: Wagtail feature map

As of 2026-10-22. Living copy: https://claude.ai/artifact/6UgC5cCqTRURzSEspGoh7R

## Summary

No Wagtail feature is built yet: the swago repo (`~/Projects/swago`, one commit on `main`) holds only `docs/DESIGN.md`, with no Go, templ or htmx code. So this map compares Wagtail against the design, not against code. Of 28 features (27 core Wagtail ones plus multi-tenancy), 7 have a concrete design decision, 11 are named with no design, and 10 are not mentioned at all. Since this map was first written, DESIGN.md has settled the query layer (Ent), migrations (Atlas), sites and routing, and first-class multi-tenancy. The biggest remaining risks are page field storage and how the htmx admin edits rich text and StreamField.

## Feature map

This is a living document: each milestone adds or revises rows as features are designed and built, and a feature is not done until it works from a scaffolded project.

Status means: **Designed** = DESIGN.md makes a concrete decision; **Named** = listed in the parity checklist or build order with no design; **Not covered** = absent from DESIGN.md. Nothing is **Built** yet.

| Wagtail feature | Status | Proposed Go approach |
| --- | --- | --- |
| Page tree (treebeard) | Designed | Materialized `path`, `depth`, `numchild` columns as in DESIGN.md. A `tree` package does add, move and copy inside one transaction. |
| Typed page models | Designed | Structs implementing a `Page` interface, registered with `Register[T]()`. Storage layout (table per type vs JSON column) still open; Ent has no inheritance, so a page type is an entity linked to a base page entity. |
| StreamField blocks | Designed | Same `{type, id, value}` JSON as Wagtail. A `Block` interface with a type registry for unmarshalling, and a `Render` method returning a `templ.Component`. |
| Database backends | Designed | SQLite default, Postgres and MySQL later. Use the pure-Go `modernc.org/sqlite` driver so the binary stays CGO-free. Query layer: Ent (decided 2026-10-22). |
| Front-end rendering | Designed | Each page type returns a templ component. DESIGN.md still lists templ vs `html/template` as open; the project goal already says templ. |
| Revisions, draft and publish | Named | `page_revisions` table holding a JSON snapshot per save. Page row keeps `live_revision_id`, `latest_revision_id` and `has_unpublished_changes`. |
| Headless API | Named | JSON handlers on `net/http` for pages, images and documents, reusing the revision serializer. |
| Users, groups, permissions | Named | Session cookies, argon2id hashes, groups. Page permissions are granted on a tree node and checked as a `path` prefix match. |
| Admin UI | Named | templ + htmx: explorer as `hx-get` partials, edit forms generated from struct tags, modal choosers. DESIGN.md still lists htmx vs SPA as open. |
| Images and renditions | Named | Storage interface (local disk, S3). Filter specs like `fill-300x200` produce cached renditions; focal point stored per image. Pure-Go `imaging` vs CGO `govips` to decide. |
| Documents | Named | Same storage interface; downloads go through a handler that checks collection permissions. |
| Search | Named | `Searcher` interface with a database backend per dialect (SQLite FTS5, Postgres `tsvector`, MySQL FULLTEXT). Bleve or Meilisearch as optional backends. |
| Workflows and moderation | Named | Workflow, task and task-state tables, driven by a small state machine. |
| Localization (i18n) | Named | `locale` and `translation_key` on pages, as in Wagtail core. Admin strings via `go-i18n`. |
| Form builder | Named | A form page type with field blocks and a submissions table, plus CSV export. |
| Snippets | Named | Any registered struct gets admin list, edit and chooser views. |
| Install, scaffold and extend (`wagtail start`) | Designed | `swago` library plus CLI; `swago new` scaffolds a project that registers its own pages and blocks. Ent codegen across core and project schemas needs a spike. See DESIGN.md. |
| Multi-tenancy | Designed | Shared database with `tenant_id` on every content table via an Ent mixin, enforced by interceptors, privacy rules and composite keys, with a leak test over every model. Scope sits behind `tenant.Scope`. |
| Rich text | Not covered | Store HTML with Wagtail-style link references (`<a linktype="page" id="3">`), resolved at render. Sanitize with `bluemonday`. Editor: a JS editor (Tiptap or Trix) mounted inside the htmx form. |
| Sites and URL routing | Designed | Wagtail's Site model (hostname, port, root page, default site) and matching order are in DESIGN.md. Still to design: the `Route` method on a page type that replaces RoutablePageMixin. |
| Schema migrations | Designed | Ent auto-migrate in dev, Atlas versioned files in production, expand-and-contract for renames and drops, and a CI check that the schema diff is empty. |
| Preview | Not covered | Render an unsaved form state through the page's templ component in a side panel. |
| Scheduled publishing | Not covered | `go_live_at` and `expire_at` on revisions, applied by a background ticker. |
| Redirects | Not covered | Redirects table checked by middleware before returning a 404. |
| Site settings | Not covered | Typed settings structs registered per site, edited in the admin. |
| Embeds (oEmbed) | Not covered | oEmbed fetcher with a cache table, used by rich text and an embed block. |
| Sitemaps | Not covered | `sitemap.xml` generated from live pages per site. |
| Audit log | Not covered | Log-entry table written on every admin action. |
| Locking and usage tracking | Not covered | Lock fields on pages; a reference index recording which pages use each image, document and snippet. |

## Decisions that block building

These decisions need answers before the first line of page-tree code. Migrations, sites and routing, and the query layer are now settled in DESIGN.md (Atlas, Site model, Ent); the rows below keep their original options for reference.

| Decision | Options | Recommendation |
| --- | --- | --- |
| Page field storage | One table per page type joined to a base `pages` table (Wagtail's multi-table inheritance), or one `pages` table with a JSON `data` column | Base `pages` table for tree, URL and publish state, plus a JSON `data` column for type fields. It matches the revision snapshot format, works on all three databases and needs no per-type migrations. Promote a field to a real column only when it must be queried or indexed. |
| Migrations | Hand-written SQL, goose, Atlas, or codegen | Settled: Atlas versioned migrations generated from the Ent schema (DESIGN.md leans Atlas over goose). |
| Sites and routing | Single site, or Wagtail-style multi-site | Add a `sites` table from day one and store `url_path` on each page. Retrofitting multi-site later means rewriting every URL. |
| Rich text format | Wagtail-style HTML with link references, or editor JSON (ProseMirror) | HTML with link references: readable, easy to import from Wagtail, and renderable without the editor. |
| StreamField editing under htmx | Server-rendered block forms swapped by htmx, or one JS island for the whole stream | Server-rendered block forms, with htmx for add, delete and reorder (`hx-post` returning one block's form). Keep JS to the rich text editor and drag-and-drop. Prototype this early; it is the riskiest part of the admin. |
| Query layer, module path, Go version | bun vs sqlx; module path and Go version unset | Settled: Ent. Module path and Go version are still unchosen; pick Go 1.25 or later. |

templ vs `html/template` and htmx vs SPA are closed in DESIGN.md (templ and htmx).

## Suggested build order

The order keeps DESIGN.md's sequence but adds a foundations step, pulls sites and routing into the page tree, and moves templ rendering and an admin spike ahead of the headless API. Each milestone ends with something you can run.

1. **Foundations.** Settle the open decisions above, `go mod init`, Ent with SQLite and the `tenant_id` mixin, the tenant leak test, Atlas migrations, and a test harness that runs against SQLite (Postgres in CI later).
    - Also: a spike proving a scaffolded project can generate Ent code that includes the core schemas.
    - Done when: `swago new mysite` produces a project whose `swago migrate` creates the core tables on an empty SQLite file.
2. **Page tree, page models, sites and routing.** `pages` table with materialized paths, `Register[T]()`, `sites` table, host-to-page middleware, `url_path` kept in sync on move.
    - Done when: two page types seeded from code are served at their URLs.
3. **Front-end rendering with templ.** Page and block rendering through templ components, base layout, static files.
    - Done when: a home page and child pages render from the tree.
4. **Blocks, rich text and revisions.** Block registry and JSON (de)serialization, rich text storage with link references, `page_revisions`, draft, publish and unpublish.
    - Done when: a page with a StreamField body is saved as a draft, published, and the old revision restored.
5. **Auth and permissions.** Users, groups, sessions, login, CSRF, and page permissions on tree nodes.
    - Done when: an editor can edit only their section of the tree.
6. **Admin UI with htmx.** Explorer, generated edit forms, the StreamField editor, page chooser, preview, revision history. Start a throwaway StreamField-editor spike alongside step 2, since it is the riskiest piece.
    - Done when: a non-developer can create, edit and publish a page without the CLI.
7. **Media and snippets.** Images with renditions and focal points, documents, collections, snippets, reference index.
    - Done when: an image block shows a cropped rendition and the image's usage page lists the pages using it.
8. **Headless API and search.** JSON API for pages, images and documents; database search backends.
    - Done when: `/api/v2/pages/?search=` returns live pages on SQLite and Postgres.
9. **Publishing extras.** Scheduled publishing, redirects, site settings, sitemaps, embeds, audit log, locking.
10. **Larger features.** Workflows and moderation, localization, form builder, Wagtail importer, MySQL backend.

Steps 1 to 6 are the usable core the design doc estimates in months; steps 7 to 10 can run in any order after it.
