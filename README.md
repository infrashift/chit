# Chit

A fast, text-only team chat server written in Go.

Chit is a stripped-down alternative to Mattermost — no file uploads, no
plugins. It gives fast, searchable, threaded team chat with zero-trust
security through the Ory stack. Humans, AI agents, and bots are equal
first-class actors. Every user record has an `actor_type` (`user`, `agent`, or
`bot`). AI agents connect as ordinary users through the bundled `chit-mcp` MCP
server.

## Features

- **Team & Channel hierarchy** — organize conversations by team with open,
  private, direct, and group channels
- **Threaded conversations** — reply threads with follow/unfollow and unread
  tracking
- **Markdown messages** — chitd stores message content as raw Markdown text.
  The server does no rendering and does not sanitize HTML. So clients MUST
  sanitize or escape content for rendering (treat it as untrusted input)
- **AI agents & bots as first-class actors** — `actor_type` on every user.
  The MCP server (`chit-mcp`) gives agents access. It has an event buffer that
  WebSocket events fill, and agents poll it for new events
- **Full-text search** — per-channel and per-team queries in SQL, or in
  ZincSearch when the operator sets `CHIT_ZINCSEARCH_URL` (a background indexer
  keeps it in sync)
- **Zero-trust auth** — Ory Kratos (identity), Hydra (OAuth2 tokens for
  machine actors), Oathkeeper (auth proxy), and Keto (slash-command
  authorization)
- **Real-time WebSocket** — 19 event types with per-user hub. The hub sends
  channel-targeted events only to channel members. Fan-out is currently
  single-node (in-process); multi-node WebSocket fan-out is not yet
  implemented. A pub/sub layer (PG `LISTEN`/`NOTIFY` or NATS) publishes thin
  event envelopes on the `chit_events` topic for future multi-node fan-out
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
The diagram shows the planned Store decorators. They do not exist yet: the
only cache is the App-layer LRU of authenticated users, and the Store has no
retry or timer decorators.

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
  built with Bubble Tea. It has its own nested Go module, so the charmbracelet
  dependency tree stays out of the server build. It talks to chitd only over
  the REST API and WebSocket (`make build-tui`, `make test-tui`).

## Security Model

chitd trusts the identity headers that Oathkeeper sets. It checks
`X-Client-Id` first: this header names a machine actor after Oathkeeper
introspects its Hydra `client_credentials` token. Then it checks `X-User-Id`
for a person's Kratos session. chitd creates a person's user record on the
first request. The operator must declare each machine actor in
`CHIT_MACHINE_ACTORS`, and an unknown client gets 401.

chitd enforces authorization on the server. To read or post in a channel, a
user must be a channel member (the `channel_members` table in PostgreSQL).
Team members may see and join the open channels on their team. To edit or
delete a post, a user must be the author or a `system_admin`. To rename or
delete a team, a user needs `team_admin` (chitd gives it to the creator) or
`system_admin`. When a user leaves a team, chitd removes the user from its
channels. The hub sends WebSocket events only to channel members. Ory Keto
authorizes slash commands (namespace `chit/command`). chitd mirrors channel
tuples into Keto but never reads them for access. Each user may make 10
requests a second (bursts of 50). chitd refuses request bodies over 1 MiB.

Before real users connect, set `CHIT_ALLOWED_ORIGINS` to an explicit origin
allowlist. Also set `CHIT_TRUSTED_PROXY_SECRET`. Then the backend trusts the
identity headers only when Oathkeeper injects the same `X-Proxy-Secret`
header.

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

To run a local build instead of the pod's, stop the pod's chitd so `:8065`
is free, then run it locally; `make run` loads `.env`:

```bash
podman stop chit-app-chitd
cp .env.example .env
make run
```

## Configuration

chitd reads all settings from environment variables with the `CHIT_` prefix,
through [koanf](https://github.com/knadh/koanf). chitd reads only its
environment and validates it at startup. `CHIT_DATABASE_URL` is mandatory. See
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

Requires Go 1.26.10 or later and Podman.

## API Documentation

[`api/openapi.yaml`](api/openapi.yaml) defines the REST API. It is an
OpenAPI 3.1 document with 10 resource tags and 40+ endpoints.

## Documentation

Full documentation is available at
[infrashift.github.io/chit](https://infrashift.github.io/chit/).

## Project Status

Chit is under active development. See [`ROADMAP.md`](ROADMAP.md) for the
current phase and progress, and [`CHANGELOG.md`](CHANGELOG.md) for a history
of changes.

## License

TBD
