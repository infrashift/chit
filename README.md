# Chit

A high-performance, text-only team chat server built with Go.

Chit is a stripped-down alternative to Mattermost — no file uploads, no bots,
no plugins. Just fast, searchable, threaded team messaging with zero-trust
security via the Ory stack.

## Features

- **Team & Channel hierarchy** — organize conversations by team with open,
  private, direct, and group channels
- **Threaded conversations** — reply threads with follow/unfollow and unread
  tracking
- **Markdown messages** — full CommonMark support in all posts
- **Full-text search** — ZincSearch-backed indexing with per-channel and
  per-team queries
- **Zero-trust auth** — Ory Kratos (identity), Oathkeeper (auth proxy), and
  Keto (ReBAC authorization)
- **Real-time WebSocket** — 14 event types with per-user hub and broadcast
  filtering
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

## Quick Start

```bash
# Clone the repo
git clone https://github.com/infrashift/chit.git
cd chit

# Start infrastructure (PostgreSQL, Ory stack, ZincSearch)
make kube-up

# Run Ory schema migrations
make kube-migrate

# Run Chit schema migrations
make migrate-up

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
OpenAPI 3.1 spec covering 9 resource tags and 40+ endpoints.

## Documentation

Full documentation is available at
[infrashift.github.io/chit](https://infrashift.github.io/chit/).

## Project Status

Chit is under active development. See [`ROADMAP.md`](ROADMAP.md) for the
current phase and progress, and [`CHANGELOG.md`](CHANGELOG.md) for a history
of changes.

## License

TBD
