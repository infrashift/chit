# Chit

A high-performance, text-only team chat server built with Go.

Chit is a stripped-down alternative to Mattermost — no file uploads, no
plugins. Just fast, searchable, threaded team messaging with zero-trust
security via the Ory stack. Humans, AI agents, and bots are equal first-class
actors: every user record carries an `actor_type` (`user`, `agent`, or `bot`),
and AI agents connect through the bundled `chit-mcp` MCP server as ordinary
users.

## Features

- **Team & Channel hierarchy** — organize conversations by team with open,
  private, direct, and group channels
- **Threaded conversations** — reply threads with follow/unfollow and unread
  tracking
- **Markdown messages** — message content is stored as raw markdown text; the
  server performs no rendering or HTML sanitization, so clients MUST sanitize
  or escape content when rendering (treat it as untrusted input)
- **AI agents & bots as first-class actors** — `actor_type` on every user,
  MCP server (`chit-mcp`) for agent access, with a WebSocket-fed event
  buffer agents can poll for new events
- **Full-text search** — ZincSearch-backed indexing with per-channel and
  per-team queries
- **Zero-trust auth** — Ory Kratos (identity), Oathkeeper (auth proxy), and
  Keto (ReBAC authorization)
- **Real-time WebSocket** — 14 event types with per-user hub; channel-targeted
  events are delivered only to channel members. Fan-out is currently
  single-node (in-process); multi-node WebSocket fan-out is not yet
  implemented. A pub/sub layer (PG LISTEN/NOTIFY or NATS) publishes thin event
  envelopes on the `chit_events` topic for future multi-node fan-out
- **Conversation tags** — label and filter posts with user-defined tags
- **Message pinning** — pin important posts per channel
- **@mention notifications** — real-time mention alerts via WebSocket

## Architecture

```
TUI Client ──► Ory Oathkeeper ──► REST API ──► App Layer ──► Store ──► PostgreSQL
                    │                              │            │
                    │                              ▼            ├──► LRU Cache
                    │                          WebSocket Hub    └──► Timer/Retry
                    │                              │                  Decorators
                    ▼                              ▼
               Ory Kratos                    PG LISTEN/NOTIFY
               Ory Keto                      (or NATS)
```

Layered design inspired by Mattermost: **API → App → Store → PostgreSQL**.
Store decorators add caching, retry logic, and timing instrumentation
transparently.

The root module builds four binaries:

- **`chitd`** — the chat server
- **`chit-mcp`** — MCP stdio server that lets AI agents act as normal users;
  connects to chitd over REST/WebSocket (no database access), like
  chit-claude
- **`chit-reconcile`** — one-shot job that pushes CUE-defined roles, commands,
  and actors into Keto
- **`chit-claude`** — bridge that drives headless Claude Code sessions from
  Chit channels (thread = session; see
  [Headless Claude Code](docs/src/content/docs/deployment/headless-claude.mdx))

## Clients

- **`chit-tui`** ([clients/chit-tui](clients/chit-tui/)) — terminal client
  built with Bubble Tea. Lives in its own nested Go module so the
  charmbracelet dependency tree stays out of the server build; talks to chitd
  exclusively over the REST API and WebSocket (`make build-tui`,
  `make test-tui`).

## Security Model

Channel authorization is enforced server-side: reading or posting to a
channel requires channel membership (checked against the `channel_members`
table in PostgreSQL), editing or deleting a post requires being the author or
a `system_admin`, and WebSocket events are delivered only to channel members.
Ory Keto remains the authorization backend for slash commands (namespace
`chit/command`). For production deployments, set `CHIT_ALLOWED_ORIGINS` to an
explicit origin allowlist and `CHIT_TRUSTED_PROXY_SECRET` so the backend only
trusts the `X-User-Id` header when Oathkeeper injects a matching
`X-Proxy-Secret` header.

## Quick Start

```bash
# Clone the repo
git clone https://github.com/infrashift/chit.git
cd chit

# Start infrastructure (PostgreSQL, Ory stack, ZincSearch)
# Kratos, Keto, and Chit schema migrations run automatically via
# init/migrate containers.
make kube-up

# (Later, only when new migration files are added) re-apply Chit
# schema migrations against the running pod without restarting it
make kube-migrate

# Configure environment
cp .env.example .env

# Build and run
make run

# Verify
curl http://localhost:8065/api/v1/system/ping
```

## Configuration

All settings are configured via environment variables with the `CHIT_` prefix
using [koanf](https://github.com/knadh/koanf). See
[`.env.example`](.env.example) for the full list of variables.

## API Documentation

The REST API is defined in [`api/openapi.yaml`](api/openapi.yaml) — an
OpenAPI 3.1 spec covering 10 resource tags and 40+ endpoints.

## Documentation

Full documentation is available at
[infrashift.github.io/chit](https://infrashift.github.io/chit/).

## Project Status

Chit is under active development. See [`ROADMAP.md`](ROADMAP.md) for the
current phase and progress, and [`CHANGELOG.md`](CHANGELOG.md) for a history
of changes.

## License

TBD
