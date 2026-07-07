# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `chit-tui` terminal client imported into the monorepo at `clients/chit-tui`
  as a nested Go module (own `go.mod`, own dependency tree) with a committed
  root `go.work` workspace. Server builds and images exclude it. New Makefile
  targets: `build-tui`, `test-tui`, `lint-tui`, `test-e2e-tui`, `test-all`.
  TUI docs merged into the main docs site under a "TUI Client" section.
- Global post search endpoint `POST /api/v1/posts/search` — searches across
  every channel the caller is a member of (no team/channel scope)
- `internal/chitclient`: shared HTTP/WebSocket client of chitd used by the
  headless agent binaries (extracted from `internal/bridge` and extended
  with the full REST surface the MCP server needs)
- `make check-deps` dependency-boundary guards (run in CI): client binaries
  must never link server-side deps, chitd must not link the MCP SDK, and
  TUI/charmbracelet deps must stay out of the server module

- `chit-claude` bridge (`cmd/chit-claude`, `internal/bridge`): drives headless
  Claude Code sessions from Chit channels. Thread = session (resumed via
  `claude -p --resume`, session ID stored in reply post props), per-thread run
  serialization, token/cost/context footer from the run's JSON output, and
  error replies on failed runs. Connects to chitd over REST/WebSocket as the
  agent user — no database access. See `docs/.../deployment/headless-claude.mdx`.

- Slash commands framework: CUE-defined commands and roles, Keto-backed
  authorization (`chit/command` namespace, `execute` relation), audit
  logging, and a CloudEvents webhook dispatcher; `chit-reconcile` binary
  pushes CUE definitions into Keto
- Containerized Ory stack (Kratos, Oathkeeper, Keto) in the Podman Kube
  manifests, plus a full UAT environment with pre-seeded users
- `actor_type` column on users (`user` | `agent` | `bot`) — migration
  `000004_actor_type`; humans, AI agents, and bots are equal first-class
  actors
- MCP agent event feed: chit-mcp buffers real-time events for agents to
  poll via the `get_new_events` tool
- Background search indexer: polls posts by `update_at` watermark every 5
  seconds and upserts documents into ZincSearch (keyed by post ID);
  soft-deleted posts are removed from the index; SQL ILIKE fallback when
  ZincSearch is unavailable
- GitHub Actions CI workflow: build + unit tests with race detector,
  golangci-lint (new issues only), integration tests against a PostgreSQL
  service container, OpenAPI + CUE validation
- `make kube-migrate` target to re-apply Chit schema migrations against the
  running pod (kube-up runs all migrations automatically)
- Layered architecture scaffold: API → App → Store → PostgreSQL
- PostgreSQL schema with UUIDv7 primary keys and bigint millisecond timestamps
  (10 tables: users, teams, team_members, channels, channel_members, posts,
  threads, thread_memberships, tags, message_tags)
- OpenAPI 3.1 specification with 40+ endpoints across 10 resource tags
- Podman Kube deployment manifest with 5 containers (PostgreSQL, Kratos,
  Oathkeeper, Keto, ZincSearch)
- Ory stack configuration files (Kratos identity schemas, Oathkeeper access
  rules, Keto namespace configuration)
- WebSocket hub with per-user connection map and single event loop goroutine
- Pub/sub abstraction with PG LISTEN/NOTIFY backend (NATS optional)
- LRU cache layer using hashicorp/golang-lru/v2
- MCP server for AI agent integration
- Containerfile for building the server image
- Makefile with build, test, lint, migrate, and kube targets
- Environment-based configuration via koanf with CHIT_ prefix (see
  `.env.example` for the full list)
- Documentation site built with Astro + Starlight
- README, ROADMAP, and CHANGELOG

### Changed

- `chit-mcp` is now an HTTP/WebSocket client of chitd (like `chit-claude`)
  instead of embedding the app/store/PostgreSQL stack in-process. Configure
  with `CHIT_MCP_SERVER_URL`, `CHIT_MCP_AGENT_KRATOS_ID`, and optional
  `CHIT_MCP_PROXY_SECRET`; `CHIT_MCP_AGENT_USER_ID` and database access are
  gone. The agent event feed now consumes chitd's WebSocket: events carry
  full payloads (`posted` events include the entire post) and membership
  filtering happens server-side in the hub. The `mark_thread_read` and
  `follow_thread` MCP tools now take a `team_id` argument.

### Security

- Channel and post authorization enforced server-side: channel membership
  required to read/post/pin, author-or-admin required to edit/delete, team
  membership required to create/join channels, invite-by-member for
  private/DM/group channels
- WebSocket events delivered only to channel members (hub membership cache)
- `CHIT_ALLOWED_ORIGINS` origin allowlist for CORS/WebSocket (wildcard logs
  a warning) and `CHIT_TRUSTED_PROXY_SECRET` to gate trust in the
  `X-User-Id` header behind an Oathkeeper-injected `X-Proxy-Secret` header
- Pagination clamped server-side (`page >= 0`, `per_page` between 1 and 200)

### Fixed

- Thread reply counts
- `since`-based post polling on `GET /channels/{id}/posts`
- Store context propagation

### Removed

- Vault Transit Engine integration and the abandoned encryption-at-rest
  column (migration `000003_drop_content_encrypted`)
- Committed `seed-uat` binary removed from git; the seeder is built from
  `scripts/seed-uat/main.go`
