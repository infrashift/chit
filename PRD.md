# Product Requirements Document (PRD): Project Chit

**Version:** 2.2

**Last Updated:** 2026-10-07

**Status:** Draft

**Project Owner:** Ryan Craig

**Core Stack:** Go, PostgreSQL, ZincSearch, Ory Stack

---

## 1. Executive Summary

**Chit** is a fast, greenfield chat application. The operator hosts it and keeps control of all data, and it can scale. The primary focus is secure text messages, with full search and fine-grained permissions. Chit uses the **Ory Stack** for identity. It uses **PostgreSQL** as the single layer that stores data and coordinates the nodes. So Chit keeps its architecture simple, and it can still grow to many nodes.

---

## 2. Project Goals

* **Zero-Trust Identity:** Offload all authentication and authorization to the Ory ecosystem.
* **Searchability:** Give sub-second full-text search across all historical conversations.
* **Simplicity:** Use PostgreSQL as the only backend, and also to coordinate the nodes. This decreases the operational work.
* **Security:** Enforce channel access server-side from PostgreSQL channel membership; authorize slash commands through Ory Keto.

---

## 3. Functional Requirements

### 3.1 Organizational Model

* **Team → Channel Hierarchy:** Teams are the top-level organizational unit. Channels belong to a team. Users join teams, then channels within those teams.
* **Direct Messages & Group Messages:** DMs and GMs are team-less channels (NULL team_id). DMs are between exactly 2 users; GMs support 3–8 users.

### 3.2 Messaging & Channels

* **Real-time Delivery:** Clients keep persistent WebSocket connections to chitd.
* **Channel Types:** Open (O), Private (P), Direct Message (D), and Group Message (G).
* **Markdown Support:** Message content can use standard Markdown.
* **Conversation Tags:** Users can apply metadata tags to messages, and then find messages by tag.
* **Message Pinning:** Users can pin important messages within a channel.
* **Durable Storage:** PostgreSQL must acknowledge each message before chitd broadcasts it to clients.

### 3.3 Threading

* **Collapsed Reply Threads:** Messages can be replies to a root post (via `root_id`).
* **Thread Tracking:** A `threads` table tracks aggregate stats (reply count, last reply time, participants) for each root post with replies.
* **Thread Memberships:** Users who participate in or follow a thread have a `thread_memberships` record tracking their read state and follow preference.
* **Auto-Follow:** A user who replies to a thread automatically follows it.

### 3.4 Search

* **Full-Text Search:** Users can search message history via keywords.
* **Filtered Search:** A user can search all the channels that the user belongs to, one team, or one channel. The user can filter by tags (all must match) and by author. The results always include only channels that the user is a member of. chitd has no filter for a time range.
* **Search Backend:** PostgreSQL by default. ZincSearch is optional (`CHIT_ZINCSEARCH_URL`). When the operator sets it, an asynchronous background worker copies posts from PostgreSQL into ZincSearch. If ZincSearch fails, search falls back to PostgreSQL.

### 3.5 Explicitly Excluded (MVP)

* File attachments
* Giphy / media embeds
* Audio/video calls
* Plugins
* Reactions / emoji reactions
* Web or native clients (the terminal client lives in this repository at `clients/chit-tui`)

> **Note:** Bots and webhooks are no longer excluded. Chit now treats humans,
> AI agents, and bots as equal first-class actors (`users.actor_type`), ships
> a slash-command framework with a CloudEvents webhook dispatcher, and
> provides MCP-based agent access. See
> [PRD-SLASH-COMMANDS.md](PRD-SLASH-COMMANDS.md) for details.

---

## 4. Ory Stack Component Mapping

Chit utilizes the Ory Stack to decouple identity concerns from the business logic.

| Component | Responsibility in Chit |
| --- | --- |
| **Ory Kratos** | **Identity Management:** Handles user registration, profile management, and MFA (YubiKey/WebAuthn). Stores the "Source of Truth" for user accounts. |
| **Ory Oathkeeper** | **Identity & Access Proxy:** Sits in front of the Go API. It validates Kratos sessions and introspects Hydra access tokens, and forwards the caller as `X-User-Id` (Kratos identity) or `X-Client-Id` (OAuth2 client). chitd trusts these headers and validates nothing itself; `CHIT_TRUSTED_PROXY_SECRET` can require the proxy to present `X-Proxy-Secret`. |
| **Ory Keto** | **Authorization (ReBAC):** Authoritative for slash-command execution only (`chit/command` namespace, `execute` relation). Channel `member` tuples are mirrored to Keto best-effort on join/leave, but channel access checks use the PostgreSQL `channel_members` table. |
| **Ory Hydra** | **OAuth2:** Deployed. Issues `client_credentials` tokens to machine actors (agents and bots), which Oathkeeper introspects (audience `chit`, scopes `chit:read`/`chit:write`). chitd maps the client ID to a user through `users.oauth_client_id`. |

### 4.1 Keto Authorization Model

Two Keto namespaces are in use:

```
namespace: chit/channel
  relation: member

namespace: chit/command
  relation: execute
```

When a user joins a channel, chitd writes a Keto relation tuple
(`chit/channel:<channel_id>#member@<user_id>`). This write is best-effort:
chitd logs a failure and does not fail the request. For a new channel, chitd
writes the tuples in the background. When the user leaves, chitd deletes the
tuple. Nothing reads these tuples for access: channel checks
(read/post/pin/invite) use the PostgreSQL `channel_members` table.

Keto authorizes each slash command. In the `chit/command` namespace, Keto
gives `Command:<id>#execute` to the subject set `Role:<name>#member`. Actors
(by chit `users.id`) are members of roles. CUE files under `auth/` define the
roles and the actor bindings, and the `chit-reconcile` binary pushes them into
Keto.

### 4.2 User Auto-Provisioning

On the first authenticated request through Oathkeeper, the Chit backend auto-provisions a local user record. To do this, it gets the identity traits from the Kratos Admin API. Chit needs no Kratos webhooks.

Chit does not add machine actors (agents and bots) automatically. The operator declares them in `CHIT_MACHINE_ACTORS`, or a system admin adds them through `POST /users`. Each machine actor links to a Hydra OAuth2 client and has no Kratos identity or email. chitd refuses an unknown OAuth2 client.

---

## 5. Technical Architecture

### 5.1 Layered Architecture (Mattermost-Inspired)

The codebase follows a layered pattern: **API → App → Store → PostgreSQL**

* **API Layer:** HTTP handlers on the chi router. This layer extracts the auth context, validates input and sends the work to the App layer.
* **App Layer:** Business logic and authorization. Coordinates between Store, Keto, WebSocket hub, and pub/sub. Holds the only cache, an LRU of authenticated users.
* **Store Layer:** Data access. The `store.Store` interface has one PostgreSQL (pgx/v5) type that satisfies it: `sqlstore`. There is no decorator chain.

### 5.2 Real-time Coordination

* **WebSocket Hub:** Per-user connection map with read/write pump goroutines per client.
* **Cross-Node Pub/Sub:** Abstracted interface with two implementations:
  * **PG `LISTEN`/`NOTIFY`** (default) — Needs no more infrastructure. It uses the PostgreSQL connection that chitd already has.
  * **NATS** (optional) — For large deployments that need a higher pub/sub throughput.
* chitd selects the pub/sub backend at startup from its configuration. chitd publishes thin event envelopes on it, but nothing consumes them yet. WebSocket delivery stays single-node until multi-node fan-out exists.

### 5.3 Search Indexing

Chit implements an "Asynchronous Worker" pattern to move data from PostgreSQL to ZincSearch:

1. chitd saves the message to PostgreSQL.
2. Every 5 seconds, a background worker reads new and changed messages after an `(update_at, id)` cursor.
3. The worker upserts each message into ZincSearch, and removes deleted ones.

In one tick, the worker handles at most 100 batches (20,000 posts). The worker runs only when the operator configures ZincSearch.

---

## 6. PostgreSQL Data Model

The schema uses UUIDv7 primary keys (time-sortable, B-tree friendly) and `BIGINT` millisecond timestamps (not `TIMESTAMP WITH TIME ZONE`).

### Core Tables

* **users** — Every actor. People link to an Ory Kratos identity (`kratos_id`); machine actors link to a Hydra OAuth2 client (`oauth_client_id`). `actor_type` is `user`, `agent` or `bot`.
* **teams** — Top-level organizational unit. Open or invite-only.
* **team_members** — Composite PK (team_id, user_id). Tracks roles and membership.
* **channels** — Belongs to a team (or NULL for DMs/GMs). Types: O, P, D, G.
* **channel_members** — Tracks per-user read state, mention counts, notification preferences.
* **posts** — Messages. A reply links to its root post through `root_id`. Content stored as TEXT (plaintext).
* **threads** — Denormalized aggregate for root posts with replies.
* **thread_memberships** — Per-user thread follow/read state.
* **tags** — Named tags that put messages into categories.
* **message_tags** — Join table that links posts to tags.

---

## 7. Security

* **In-Transit:** All traffic from client to server goes over TLS/WSS only.
* **Access Control:** Oathkeeper enforces that no request reaches the Go backend without a valid Kratos session or Hydra access token. chitd enforces channel-level access from PostgreSQL channel membership; Keto authorizes slash commands.

---

## 8. Non-Functional Requirements

* **Observability:** Write structured logs with Go's `log/slog` and JSON output.
* **Speed:** The target for message delivery latency (Send → DB → Receive) is under **200ms** for 95% of requests.
* **Portability:** The entire stack must be deployable via **Podman Kube** (`podman kube play`) for RHEL/UBI-based cloud environments.
* **Code Structure:** Use Mattermost as the model for code structure and architectural patterns.
* **ID Format:** UUIDv7 for all primary keys — time-sortable, B-tree friendly, and the nodes do not need to coordinate.
* **Router:** chi (actively maintained, standard `http.Handler` compatible).
* **Database Driver:** pgx/v5 with a connection pool.
