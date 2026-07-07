# Product Requirements Document (PRD): Project Chit

**Version:** 2.1

**Last Updated:** 2026-07-07

**Status:** Draft

**Project Owner:** Ryan Craig

**Core Stack:** Go, PostgreSQL, ZincSearch, Ory Stack

---

## 1. Executive Summary

**Chit** is a high-performance, greenfield chat application designed for sovereign, scalable communication. The primary focus is on secure text-based messaging with robust search capabilities and granular permission management. By leveraging the **Ory Stack** for identity and **PostgreSQL** as the unified persistence and coordination layer, Chit minimizes architectural complexity while maintaining a path toward high-scale distribution.

---

## 2. Project Goals

* **Zero-Trust Identity:** Offload all authentication and authorization to the Ory ecosystem.
* **Searchability:** Provide sub-second full-text search across all historical conversations.
* **Simplicity:** Utilize PostgreSQL as the sole backend and coordination engine to reduce operational overhead.
* **Security:** Enforce access control via Ory Keto for channel-level authorization.

---

## 3. Functional Requirements

### 3.1 Organizational Model

* **Team → Channel Hierarchy:** Teams are the top-level organizational unit. Channels belong to a team. Users join teams, then channels within those teams.
* **Direct Messages & Group Messages:** DMs and GMs are team-less channels (NULL team_id). DMs are between exactly 2 users; GMs support 3–8 users.

### 3.2 Messaging & Channels

* **Real-time Communication:** Support for persistent connections via WebSockets.
* **Channel Types:** Open (O), Private (P), Direct Message (D), and Group Message (G).
* **Markdown Support:** Message content supports standard markdown formatting.
* **Conversation Tagging:** Users can apply metadata tags to messages for categorized retrieval.
* **Message Pinning:** Users can pin important messages within a channel.
* **Persistence:** All messages must be acknowledged by PostgreSQL before being broadcast to clients.

### 3.3 Threading

* **Collapsed Reply Threads:** Messages can be replies to a root post (via `root_id`).
* **Thread Tracking:** A `threads` table tracks aggregate stats (reply count, last reply time, participants) for each root post with replies.
* **Thread Memberships:** Users who participate in or follow a thread have a `thread_memberships` record tracking their read state and follow preference.
* **Auto-Follow:** Replying to a thread automatically follows it.

### 3.4 Search

* **Full-Text Search:** Users can search message history via keywords.
* **Filtered Search:** Support for searching within specific channels, teams, time ranges, or by specific tags.
* **Search Backend:** ZincSearch, populated by an asynchronous background worker syncing from PostgreSQL.

### 3.5 Explicitly Excluded (MVP)

* File attachments
* Giphy / media embeds
* Audio/video calls
* Plugins
* Reactions / emoji reactions
* Web or native clients (TUI client planned separately)

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
| **Ory Oathkeeper** | **Identity & Access Proxy:** Sits in front of the Go API. It validates incoming Kratos sessions and converts them into headers (e.g., `X-User-Id`) for the backend. |
| **Ory Keto** | **Authorization (ReBAC):** Authorizes slash-command execution (`chit/command` namespace, `execute` relation). Channel `member` tuples are dual-written to Keto on join/leave, but channel access checks use the PostgreSQL membership tables as the source of truth. |
| **Ory Hydra** | **OAuth2/OIDC:** Optional for the initial MVP, reserved for future third-party integrations. |

### 4.1 Keto Authorization Model

Two Keto namespaces are in use:

```
namespace: chit/channel
  relation: member

namespace: chit/command
  relation: execute
```

When a user joins a channel, a Keto relation tuple is written:
`chit/channel:<channel_id>#member@<user_id>`; the tuple is deleted on leave.
Channel access checks (read/post/pin/invite) use the PostgreSQL
`channel_members` table as the source of truth, with Keto tuples still
dual-written.

Slash-command execution is authorized via Keto: the `chit/command` namespace
uses the `execute` relation, granted through roles defined in CUE under
`auth/` and reconciled into Keto by the `chit-reconcile` binary.

### 4.2 User Auto-Provisioning

On the first authenticated request through Oathkeeper, the Chit backend auto-provisions a local user record by fetching identity traits from the Kratos Admin API. No Kratos webhooks are needed.

---

## 5. Technical Architecture

### 5.1 Layered Architecture (Mattermost-Inspired)

The codebase follows a layered pattern: **API → App → Store → PostgreSQL**

* **API Layer:** HTTP handlers using chi router. Extracts auth context, validates input, delegates to App layer.
* **App Layer:** Business logic. Coordinates between Store, Keto, WebSocket hub, and pub/sub.
* **Store Layer:** Data access. Interface-based with PostgreSQL (pgx/v5) implementation. Decorator pattern: Timer → Retry → Cache → SqlStore.

### 5.2 Real-time Coordination

* **WebSocket Hub:** Per-user connection map with read/write pump goroutines per client.
* **Cross-Node Pub/Sub:** Abstracted interface with two implementations:
  * **PG LISTEN/NOTIFY** (default) — Keeps infrastructure minimal, uses the existing PostgreSQL connection.
  * **NATS** (optional) — Available for high-scale deployments requiring higher throughput pub/sub.
* The pub/sub backend is selected at startup via configuration.

### 5.3 Search Indexing

Chit implements an "Asynchronous Worker" pattern to move data from PostgreSQL to ZincSearch:

1. Message is saved to PostgreSQL.
2. A background worker picks up the new message.
3. The worker sends the decrypted (or specifically indexed) fields to the ZincSearch API.

---

## 6. PostgreSQL Data Model

The schema uses UUIDv7 primary keys (time-sortable, B-tree friendly) and `BIGINT` millisecond timestamps (not `TIMESTAMP WITH TIME ZONE`).

### Core Tables

* **users** — Links to Ory Kratos identity. Minimal local profile cache. Has both a chit-internal `id` (UUIDv7) and a `kratos_id` (from Ory Kratos).
* **teams** — Top-level organizational unit. Open or invite-only.
* **team_members** — Composite PK (team_id, user_id). Tracks roles and membership.
* **channels** — Belongs to a team (or NULL for DMs/GMs). Types: O, P, D, G.
* **channel_members** — Tracks per-user read state, mention counts, notification preferences.
* **posts** — Messages. Supports threading via `root_id`. Content stored as TEXT (plaintext).
* **threads** — Denormalized aggregate for root posts with replies.
* **thread_memberships** — Per-user thread follow/read state.
* **tags** — Named tags for message categorization.
* **message_tags** — Junction table linking posts to tags.

---

## 7. Security

* **In-Transit:** All client-to-server communication is strictly over TLS/WSS.
* **Access Control:** Oathkeeper enforces that no request reaches the Go backend without a valid session from Kratos. Channel-level access is enforced via Keto.

---

## 8. Non-Functional Requirements

* **Observability:** Implement structured logging using Go's `log/slog` with JSON output.
* **Performance:** Message delivery latency (Send → DB → Receive) should stay under **200ms** for 95% of requests.
* **Portability:** The entire stack must be deployable via **Podman Compose** for RHEL/UBI-based cloud environments.
* **Code Organization:** Use Mattermost for code organization and architectural patterns.
* **ID Format:** UUIDv7 for all primary keys — time-sortable, B-tree friendly, no coordination needed across nodes.
* **Router:** chi (actively maintained, standard `http.Handler` compatible).
* **Database Driver:** pgx/v5 with connection pooling.
