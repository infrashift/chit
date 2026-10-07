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
- **Full-text search** — per-channel and per-team queries in SQL, or in
  ZincSearch when `CHIT_ZINCSEARCH_URL` is set (a background indexer keeps it
  in sync)
- **Zero-trust auth** — Ory Kratos (identity), Hydra (OAuth2 tokens for
  machine actors), Oathkeeper (auth proxy), and Keto (slash-command
  authorization)
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
               Ory Hydra                     (or NATS)
               Ory Keto
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

chitd trusts identity headers set by Oathkeeper: `X-Client-Id` for a machine
actor whose Hydra `client_credentials` token was introspected, checked first,
or `X-User-Id` for a person's Kratos session. A person's user record is
created on their first request; a machine actor must be declared in
`CHIT_MACHINE_ACTORS`, and an unknown client gets 401.

Authorization is enforced server-side. Reading or posting to a channel
requires channel membership (the `channel_members` table in PostgreSQL);
team members may see and join the open channels on their team. Editing or deleting a post
requires being the author or a `system_admin`; renaming or deleting a team
requires `team_admin` (granted to its creator) or `system_admin`. Leaving a
team removes you from its channels. WebSocket events are delivered only to
channel members. Ory Keto authorizes slash commands (namespace
`chit/command`); channel tuples are mirrored into it but never consulted for
access. Each user may make 10 requests a second (bursts of 50), and request
bodies over 1 MiB are refused.

For production deployments, set `CHIT_ALLOWED_ORIGINS` to an explicit origin
allowlist and `CHIT_TRUSTED_PROXY_SECRET` so the backend only trusts the
identity headers when Oathkeeper injects a matching `X-Proxy-Secret` header.

## Quick Start

```bash
# Clone the repo
git clone https://github.com/infrashift/chit.git
cd chit

# Build the chitd image and start the stack: PostgreSQL, the Ory stack
# (Kratos, Hydra, Oathkeeper, Keto), ZincSearch and chitd on :8065.
# Kratos, Keto, Hydra, and Chit schema migrations run automatically in
# init containers.
make kube-up

# Verify (direct to chitd; through Oathkeeper on :4455 every /api/v1
# route needs a session or token)
curl http://localhost:8065/api/v1/system/ping

# (Later, only when new migration files are added) re-apply Chit
# schema migrations against the running pod without restarting it
make kube-migrate
```

To run your own build instead of the pod's, stop the pod's chitd so `:8065`
is free, then run it locally; `make run` loads `.env`:

```bash
podman stop chit-app-chitd
cp .env.example .env
make run
```

## Configuration

All settings are configured via environment variables with the `CHIT_` prefix
using [koanf](https://github.com/knadh/koanf). chitd reads only its
environment and validates it at startup; `CHIT_DATABASE_URL` is required. See
[`.env.example`](.env.example) for the variables and the
[configuration reference](docs/src/content/docs/reference/configuration.mdx)
for defaults and validation rules.

## Development

```bash
make test               # unit tests (-race)
make test-integration   # store/pub-sub tests against a throwaway PostgreSQL in Podman
make lint               # golangci-lint
make vuln               # govulncheck
```

Requires Go 1.25.14 or later and Podman.

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
