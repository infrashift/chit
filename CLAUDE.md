# CLAUDE.md — chit

## Purpose (WHY)
chit is a text-only team chat server in Go. People, AI agents and bots are users of the same kind. The Ory stack authenticates each request. chitd checks the permissions.

## Tech Stack (WHAT)
- **Language:** Go 1.26.10. `go.work` joins the server module (`.`) and the TUI module (`clients/chit-tui`).
- **Server:** chi, pgx (PostgreSQL), koanf, CUE, Ory Kratos, Hydra, Oathkeeper and Keto (keto-cue).
- **TUI:** Bubble Tea in a separate module ([ADR-0008](.okfw/decisions/adr-0008-chit-tui-is-a-separate-go-module--01a1f5379e21.md)).
- **Wiki:** okfw (`.okfw/`). See the okfw block below.
- **Docs:** Astro Starlight in `docs/`.

## Layout
- `cmd/` holds the binaries: `chitd`, `chit-mcp`, `chit-reconcile`, `chit-claude`.
- `internal/` holds the server packages. The layers are API, then App, then Store ([architecture](.okfw/architecture/chit-architecture-overview--01a1f5379dd5.md)).
- `clients/chit-tui/` holds the terminal client.
- `api/openapi.yaml` is the REST contract. `migrations/` holds the SQL migrations. `auth/` holds the CUE roles and commands.
- `deploy/` holds the Podman kube files. `tests/e2e/` holds the end-to-end scenarios.

## Verification & Workflow Commands (HOW)
Run these targets after each change:
- `make lint` and `make lint-tui`: golangci-lint with the Power of 10 linters ([ADR-0004](.okfw/decisions/adr-0004-golangci-lint-enforces-the-power-of-10-linters--01a1f5379e05.md))
- `make semscope`: `semscope scan -r nasa-p10 -r .semscope.cue` on both modules ([ADR-0003](.okfw/decisions/adr-0003-semscope-checks-the-power-of-10-rules--01a1f5379dfe.md))
- `make test` and `make test-tui`: the unit tests with `-race`
- `make test-integration`: the store and pub/sub tests against PostgreSQL in Podman
- `make docs-lexis`: lexis with `vocabulary.cue` on the Markdown docs and the wiki ([ADR-0005](.okfw/decisions/adr-0005-lexis-checks-the-documentation-with-a-repository-vocabulary--01a1f5379e0c.md))
- `make wiki`: `okfw validate --all && okfw validate --okf`
- `make check`: all of the gates above, except `test-integration`

## Mandatory Execution Rules
1. **One phase at a time.** Read `PLAN.md`. Do only the first phase that has an empty box. Obey the *Development workflow* guide in `.okfw/`.
2. **Do the loop again until all gates pass.** Write the code, lint, verify. Fix each failure and run all gates again. Do not commit while a gate fails.
3. **Test integrity.** Do not weaken or delete tests to make a gate pass.
4. **Power of 10.** Obey the NASA/JPL Power of 10 rules as [ADR-0002](.okfw/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md) adapts them to Go. golangci-lint and semscope must report zero findings.
5. **Minimal abstractions.** Add no interfaces, function types or factories that nobody asked for.
6. **The wiki stays current.** Each `PLAN.md` phase has a mirror issue in `.okfw/issues/`. Record each decision in an ADR when you make it. Close the issue when you complete the phase.
7. **Documentation obeys ASD-STE100 (Simplified Technical English).** Use the active voice, the simple present tense and short sentences. Run `make docs-lexis`.

<!-- BEGIN okfw -->
## Project knowledge base (okfw)

chit keeps a local-first OKF wiki in `.okfw/`: markdown concepts with
YAML frontmatter, linked to each other. The files are the whole database —
search and the link graph are computed from them on demand, so there is no
index to build and nothing to sync. Each directory's `index.md` is generated
and gives a cheap overview before you open anything (OKF section 8).

- **Search before you derive.** `okfw search "<query>"` / `okfw_search`. The
  wiki holds what the code does not say — why things are built the way they
  are, what was considered and rejected, dependency and security findings,
  operational context, open work. Check it before explaining a design,
  assessing risk, or running an analysis someone may already have recorded and
  adjudicated.
- **Browse top-down.** Each directory's `index.md` lists its concepts with
  their descriptions (`okfw_read {"ref":"<dir>/"}` over MCP). Open only what fits.
- **Search before you write**, too, so a new concept links to what already
  exists instead of duplicating it.
- **Link actively** — `[Title](/dir/file.md)` from the wiki root, or
  `./file.md`. Links are what makes the wiki more than a folder.
- **One idea per concept**, with a one-sentence `description`.
- **Use `okfw move` / `okfw delete`, never `mv` / `rm`.** `move` repairs every
  link to the concept and its own relative links, and both keep the `index.md`
  listings current; the shell commands break the links and leave the listings
  stale.
- **`status` is the OKF lifecycle field** (`draft`/`stable`/`deprecated`), not
  task state. Mark superseded knowledge rather than deleting it.
- **Custom metadata goes under `x:`** — the schema is closed and rejects any
  other unrecognized key.
- Don't edit `index.md` or `log.md`; okfw generates them.

Results are capped and say so: `truncated`, or `… N more`, means you did not
see everything — raise the limit or page with `--offset`/`"offset"`.

`okfw mcp` serves all of this as MCP stdio tools named `okfw_*`.
<!-- END okfw -->
