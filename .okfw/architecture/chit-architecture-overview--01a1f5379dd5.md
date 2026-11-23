---
x-okfw-id: "01a1f537-9dd5-76af-9c7b-61622fc40fa8"
title: "chit architecture overview"
type: "Architecture"
tags:
  - "architecture"
  - "overview"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T13:10:25-06:00"
description: "The chit binaries, the API to App to Store layers, the Ory security stack, the WebSocket hub and the pub/sub topic."
---

# chit architecture overview

chit is a text-only team chat server in Go. People, AI agents and bots are users of the same kind. Each user record has an `actor_type`: `user`, `agent` or `bot`.

## Binaries
The root Go module builds four binaries from `cmd/`:

| Binary | Job |
|---|---|
| `chitd` | The chat server: REST API, WebSocket hub, background jobs. |
| `chit-mcp` | An MCP stdio server. An AI agent uses it to act as a normal user. It talks to chitd over REST and WebSocket and has no database access. |
| `chit-reconcile` | A one-shot job. It writes the roles, commands and actors of the CUE files in `auth/` to Keto. |
| `chit-claude` | A bridge. It runs headless Claude Code sessions from chit channels. One thread is one session. |

`clients/chit-tui` is a terminal client with Bubble Tea. It is a separate Go module, so its charmbracelet dependencies stay out of the server build ([ADR-0008](/decisions/adr-0008-chit-tui-is-a-separate-go-module--01a1f5379e21.md)). It talks to chitd over REST and WebSocket only.

## Layers
```
chit-tui ──► Oathkeeper ──► REST API ──► App ──► Store ──► PostgreSQL
                │                         │
                ▼                         ▼
     Kratos, Hydra, Keto            WebSocket hub ──► clients
                                          │
                                  pub/sub (chit_events)
```

| Package | Job |
|---|---|
| `internal/api` | HTTP routes (chi), middleware, request validation, the OpenAPI document. |
| `internal/app` | Business rules: permissions, provisioning, posts, channels, tags, search, slash commands. |
| `internal/store` | The `Store` interface and the PostgreSQL implementation (pgx). `storetest` is an in-memory fake for tests. |
| `internal/websocket` | The hub: one event loop goroutine and one read pump and one write pump for each connection. |
| `internal/pubsub` | Thin event envelopes on the topic `chit_events`, with PG LISTEN/NOTIFY or NATS ([ADR-0007](/decisions/adr-0007-thin-pub-sub-envelopes-with-pg-listen-notify-by-default--01a1f5379e1a.md)). |
| `internal/jobs` | Background workers: the search indexer (ZincSearch), the webhook dispatcher (CloudEvents) and the file sweep. |
| `internal/keto` | The Keto client: slash-command permission and the channel tuple mirror. |
| `internal/blob` | A small S3 client with SigV4. It keeps the bytes of the attachments in SeaweedFS. |
| `pkg/imagecheck` | Checks and encodes again each uploaded image. It has no chit dependencies, so HALO can use it too. |
| `internal/memberhook` | Asks the membership policy hook before each channel add, with a Hydra token. |
| `internal/command` | The slash-command parser and the CUE command definitions. |
| `internal/mcp`, `internal/chitclient` | The tools of chit-mcp and its REST and WebSocket client. |
| `internal/bridge` | The chit-claude bridge. |
| `internal/config` | Environment configuration (koanf, prefix `CHIT_`). chitd validates it at startup. |
| `internal/model` | Data types and their validation. |

## Security
Oathkeeper is in front of chitd ([ADR-0006](/decisions/adr-0006-the-ory-stack-authenticates-and-chitd-authorizes--01a1f5379e13.md)). chitd trusts the identity headers that Oathkeeper sets. `X-Client-Id` names a machine actor with a Hydra `client_credentials` token. `X-User-Id` names a person with a Kratos session. chitd makes the user record of a person on the first request. A machine actor must be in `CHIT_MACHINE_ACTORS`.

chitd checks permissions itself. PostgreSQL holds the channel membership. Keto authorizes slash commands only. An allowlisted machine actor can act as a person with the `X-On-Behalf-Of` header ([On-behalf-of requests](/specs/on-behalf-of-requests--01a1f5379dea.md)).

## Limits
- A request body has a maximum of 1 MiB.
- Each person, and each machine actor that is not in `CHIT_ON_BEHALF_OF_CLIENTS`, can make 10 requests each second, with bursts of 50. `CHIT_RATE_LIMIT_USER_*` changes these values.
- An on-behalf-of client has an aggregate limit: 1,500 requests each second, with bursts of 3,000. `CHIT_RATE_LIMIT_CLIENT_*` changes these values.
- An upload has a maximum of 10 MiB. Each person has a quota of 1 GiB (`CHIT_FILE_QUOTA_BYTES`).
- A client WebSocket message has a maximum of 4096 bytes. Each connection has a send buffer of 256 events. The hub closes a client with a full buffer.
- The hub queue holds 256 broadcasts.

## Events
The hub sends events to the members of a channel or team only ([WebSocket events](/specs/websocket-events--01a1f5379de0.md)). WebSocket fan-out is in-process on one node. chitd also publishes thin envelopes on `chit_events`, but no component reads them yet.

## Related
- REST API: [REST API contract](/specs/rest-api-contract--01a1f5379dda.md)
- Slash commands: [Slash commands](/specs/slash-commands--01a1f5379de5.md)
- Power of 10 rules: [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
