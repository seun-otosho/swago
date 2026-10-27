# swago: session notes (resume here)

Rewritten 2026-10-27. The previous version of this file (never committed) described the
state as of 2026-10-22, before any Go code existed — that's long obsolete. **This version
reflects the actual repo at commit `99ab047` on `main`, pushed to `origin`.**

`docs/DESIGN.md` and `AGENTS.md` are the living sources of truth and are kept up to date
after every change in this session — if anything here disagrees with them, trust them. This
file exists for narrative continuity: what happened, across two different AI sessions
working the same repo, and what to do next.

## The two-session history (important context)

This repo was worked by **two separate Claude Code sessions concurrently in the same local
checkout**, around 2026-10-22–23:

1. **A session running inside the Antigravity IDE** (`~/.claude/projects/-home-seunleke-Projects-swago/bfbbf0c9-....jsonl`)
   wrote the original `docs/DESIGN.md`/`docs/FEATURE_MAP.md`, then built the foundation:
   tenant scoping (`tenant/`), the core `Page` schema, materialized-path helpers
   (`tree/path.go`), the DB opener (`db/`), `examples/site/`, and finally the page-tree
   service (`tree/service.go`, `examples/site/store.go`). It committed that last piece as
   `c16f1ca`, then got interrupted mid-instruction ("start there and keep going" on the
   `Site` entity) before writing any more code.
2. **This session (omp/Claude Code, not Antigravity)** wrote `AGENTS.md`, found the other
   session's unpushed `c16f1ca` commit sitting in the same local git history, pushed it, and
   then continued the build order from exactly where it stopped — five more commits, one
   build-order item each, every one SQLite+Postgres-tested before pushing.

If you start a third session: it's plain Go code now, not a two-session coordination
problem. Just read `AGENTS.md` + `docs/DESIGN.md` like normal.

## Current state: what's built (commit `99ab047`)

| Commit | What |
| --- | --- |
| `ca2ff91` | Foundations: `tenant/` (scoping, interceptor, hook, composite FKs), `schema/page.go` (core `Page`), `tree/path.go` (materialized paths), `db/` (SQLite/Postgres opener), `examples/site/` |
| `c16f1ca` | `tree/service.go`: transactional `Tree.AddRoot`/`AddChild`/`Move`/`Delete`, against a `tree.Store` interface; `examples/site/store.go` is the Ent-backed implementation |
| `7b73ca6` | `schema/site.go` (`Site` entity) + `routing/` package: `Resolve` (Wagtail's hostname → hostname+port → default-site order), `Middleware` (puts tenant+site in request context) |
| `ce82dbc` | `page/` package: `Register[T]()`/`Decode`, a type registry for project page structs |
| `fe6fd78` | `cmd/swago/`: `swago new` scaffolding CLI — writes a runnable Go project (go.mod, main.go with `migrate`/`serve`, the `tree.Store`/`routing.Store` implementations, `ent/schema/`) |
| `42c85b9` | `render/` package (templ): `Layout`/`Fallback`; scaffold's `serve` now resolves a request path to a page (relative to the site's root page) and renders real HTML, not JSON |
| `99ab047` | `block/` package: StreamField-style `{type,id,value}` JSON blocks, same registry convention as `page/`; `render.Stream`; reference block in `examples/site/blocks/` |

Everything is tested: `go test ./...` (SQLite) and `SWAGO_TEST_POSTGRES=... go test ./...`
both green as of the last push. `cmd/swago/scaffold_test.go` is the heaviest one — it
scaffolds a real project via a local `--replace`, builds a real binary, seeds data through
the real generated Ent client, starts the server, and asserts actual HTTP responses.

Two real bugs were only caught by running things, not by reading/compiling code in
isolation — see `docs/DESIGN.md`'s "swago new scaffolding CLI" section and the `99ab047`
commit message for specifics (a nonexistent Ent method in a hand-copied template; an
orphaned subprocess from killing `go run` instead of a built binary).

## What's NOT built yet

In FEATURE_MAP.md terms, still `Named` (no concrete schema decided) or `Not covered`:

- **Revisions, draft and publish** — `page_revisions` table, `live_revision_id`/
  `latest_revision_id`/`has_unpublished_changes` on `Page`. This is the natural next
  build-order item, but **unlike everything built so far, it has no existing design to
  follow** — it needs a real schema decision made first (and it touches the core `Page`
  entity directly), not just another additive package. Last session paused here on purpose
  rather than invent that schema unreviewed.
- **Rich text** — HTML + Wagtail-style link references, `bluemonday` sanitization. Coupled
  to an editor (Tiptap/Trix) that doesn't exist, since there's no admin yet.
- **Auth, permissions, groups** — nothing exists. `createsuperuser` in the scaffold CLI was
  explicitly *not* built because there's no `User` entity to create.
- **Admin UI (htmx)** — nothing exists. This is the biggest remaining risk DESIGN.md flags
  (nested StreamField editor, rich text editor under htmx).
- **Images, documents, search, i18n, workflows** — not started.
- **`generate` CLI subcommand** — intentionally skipped, it's just `go generate ./ent/...`,
  not worth wrapping.
- **No generic `swago.New(cfg).Register(...).Run()` app type.** DESIGN.md's illustrative
  one-liner was deliberately not built: core can't import a project's generated Ent client,
  and nothing validates what such a type should expose yet. The scaffold wires
  `open`/`Tree`/`Routing` directly instead (see `cmd/swago/main.go`'s `mainTmpl`). Revisit
  once auth/admin land.

## To resume

Read `AGENTS.md` first (repo map, conventions, commands), then `docs/DESIGN.md` (every
decision's rationale, including three completed spikes and one "built" section per
shipped feature). `docs/FEATURE_MAP.md` has the Wagtail-parity tracker and suggested build
order, though it hasn't been touched to mark things "Built" — DESIGN.md's "built" section
headers are the actual source of truth for what exists.

Verify the repo still matches this file:

```sh
git log --oneline -1          # should be 99ab047 or later
go build ./... && go vet ./... && go test ./...
```

**Most likely next step:** decide the revisions/draft-publish schema (DESIGN.md's "Page
tree" and "Sites and routing" sections show the expected shape of such a decision — a
`## Revisions, draft and publish (built ...)` section, with the same rigor: what's decided,
why, what's deliberately deferred, how it's tested), then build it the same way every prior
item was built: one core package or schema addition, a reference implementation in
`examples/site`, tests on SQLite+Postgres, `AGENTS.md`/`DESIGN.md` updated, commit, push.
