# SPECS-SLASH-COMMANDS.md

> **Status:** implemented. This spec describes the code as built
> (`internal/command`, `internal/app/command*.go`, `auth/`). Where the
> original design differed, the implementation wins and the difference is
> noted.

## 1. Data Architecture (CUE)

CUE is the single source of truth for command definitions and for which
roles may run them. Every file under `auth/` belongs to one CUE package,
`auth`, and chitd and `chit-reconcile` both load the whole tree
(`CHIT_COMMANDS_CUE_DIR`, default `auth`) and validate it before use. If the
CUE fails to load, chitd starts with slash commands disabled.

### 1.1 The Command Registry (`auth/schema/commands.cue`)

```cue
package auth

#Command: {
	id:          string & =~"^[a-z0-9-]+$"
	slug:        string & =~"^[a-z0-9-]+$"
	description: string
	category:    "chat" | "admin" | "agent"
}

registry: [string]: #Command

registry: {
	help:      {id: "help",      slug: "help",      description: "List available commands",               category: "chat"}
	invite:    {id: "invite",    slug: "invite",    description: "Invite a user or agent to the channel", category: "chat"}
	kick:      {id: "kick",      slug: "kick",      description: "Remove a user from the channel",        category: "admin"}
	topic:     {id: "topic",     slug: "topic",     description: "Set the channel topic/header",          category: "chat"}
	summarize: {id: "summarize", slug: "summarize", description: "Generate a TL;DR of channel history",   category: "agent"}
}
```

The slug carries **no leading slash**: `/invite` is looked up as `invite`.

### 1.2 Roles (`auth/schema/roles.cue` & `auth/roles/*.cue`)

```cue
package auth

#Role: {
	name:             string & =~"^[a-z0-9-]+$"
	allowed_commands: [...string]
}
```

```cue
// auth/roles/agent.cue
package auth

roles: agent: {
	name: "agent"
	allowed_commands: ["help", "invite", "summarize"]
}
```

Shipped roles:

| Role | Allowed commands |
|---|---|
| `admin` | `help`, `invite`, `kick`, `topic`, `summarize` |
| `user` | `help`, `invite`, `topic` |
| `agent` | `help`, `invite`, `summarize` |

`allowed_commands` is a list of command IDs. CUE does not check them against
the registry.

### 1.3 Actor Bindings (`auth/actors/*.cue`)

```cue
package auth

actors: "chit-agent": {
	id:    "019421a0-0000-7000-8000-00000000abcd" // chit users.id
	roles: ["agent"]
}
```

`id` is the chit **`users.id`** (not a Kratos identity ID). Humans, agents and
bots are bound the same way; actor type lives on the user row
(`users.actor_type`), not here. No actors are bound by default
(`auth/actors/README.cue` is a commented example), so until some are, every
command is denied.

---

## 2. Authorization & Intercept Logic

### 2.1 The Interceptor

Commands are intercepted in the app layer when a post is created, which means
**only through `POST /api/v1/posts`**. The WebSocket accepts no posts. In
order:

1. **Membership:** the caller must be a member of the post's channel (`403`
   otherwise), before anything else, so a command cannot be run in a channel
   the caller cannot read.
2. **Parse:** content matching `^/([a-z0-9-]+)(\s+.*)?$` is a command; the
   first group is the slug, the rest (after one whitespace character) is the
   argument string. Anything else, such as `/usr/local/bin`, is posted
   normally.
3. **Lookup:** an unknown slug answers ``Unknown command `/x`. Type `/help`
   for available commands.``
4. **AuthZ:** query Ory Keto (`CHIT_KETO_READ_URL`):
   * **Namespace:** `chit/command`
   * **Object:** `Command:<CommandID>`
   * **Relation:** `execute`
   * **Subject:** `Actor:<chit users.id>`

   A Keto error (including an error status) denies the command and answers
   ``Permission check failed for `/x`.``; a denial answers
   ``You do not have permission to use `/x`.``. Both are audited.
5. **Execute** the registered handler. A command present in CUE with no
   handler (today: `summarize`) answers ``/x` is registered but has no
   handler.``

The command is **never persisted**. `POST /posts` returns `201` with a
stand-in post (fresh `id`, `type: "command_response"`,
`props: {"ephemeral": true}`, the response text as `content`). When a handler
ran, the invoker also gets a `command_response` WebSocket event
(`{text, command_slug, channel_id}`, addressed to their `user_id`).

### 2.2 Ory Keto Relationship Mapping (ReBAC)

`chit-reconcile` pushes these tuples, all in the `chit/command` namespace:

1. **Role Membership:** `Role:<role_name>#member@Actor:<users.id>`
2. **Command Grant:** `Command:<cmd_id>#execute@(chit/command:Role:<role_name>#member)`,
   written as a Keto **subject set** so Keto expands it.

The reconciler only writes; it does not delete tuples for a removed role,
grant or binding.

### 2.3 Handlers

| Command | Behaviour |
|---|---|
| `/help` | Lists every registered command and its description. |
| `/invite <user>` | Adds the user (first word, leading `@` optional) to the channel with the same rules as `POST /channels/{id}/members`. |
| `/kick <user>` | Removes the user with the same rules as `DELETE /channels/{id}/members/{user_id}`: removing anyone but yourself also needs `system_admin`. |
| `/topic [text]` | With text, sets the channel header as `PUT /channels/{id}` would; without, reports the current header. |
| `/summarize` | Not implemented (no handler). |

Handler failures (unknown user, refused change) are returned as response text.

---

## 3. Audit Logging & Eventing

### 3.1 Structured Logging (`slog`)

A dedicated security log at `CHIT_AUDIT_LOG_PATH` (default
`/var/log/chit/audit.log`; empty or `-` writes to stderr, as does a path that
cannot be opened).

* **Format:** JSON lines (`slog.JSONHandler`), message `command_attempt`.
* **Fields:** `time`, `level`, `msg`, `event_id`, `actor_id`, `actor_type`,
  `command_slug`, `authorized` (bool).

Every command that reaches the permission check is logged, allowed or not.
Unknown commands are not.

### 3.2 Event Webhook (CloudEvents)

Enabled globally by `CHIT_WEBHOOK_ENABLED=true` with `CHIT_WEBHOOK_URL`
(required when enabled). A webhook is queued only when a handler reports a
state change: a successful `/invite`, `/kick` or `/topic <text>`. `/help`,
reading the topic, and failures send nothing.

* **Request:** `POST` to `CHIT_WEBHOOK_URL`, `Content-Type:
  application/cloudevents+json` (structured mode).
* **Headers:** `ce-specversion`, `ce-type`, `ce-source`, `ce-id`.
* **Signature:** when `CHIT_WEBHOOK_SECRET` is set, `X-Chit-Signature` is the
  lowercase hex HMAC-SHA256 of the exact request body, keyed by the secret
  (no prefix).
* **Delivery:** `CHIT_WEBHOOK_WORKER_COUNT` workers (default 4) read a queue of
  `CHIT_WEBHOOK_QUEUE_SIZE` (default 1024), each request timing out after
  `CHIT_WEBHOOK_TIMEOUT_SEC` (default 10). Best-effort: a full queue drops the
  event, and a failed or non-2xx delivery is logged and **not retried**.

```json
{
  "specversion": "1.0",
  "type": "com.chit.command.executed",
  "source": "/chit/commands",
  "id": "<the audit log event_id>",
  "time": "2026-03-03T10:00:00Z",
  "datacontenttype": "application/json",
  "data": {
    "actor_id": "<chit users.id>",
    "command_slug": "invite",
    "args": "bob",
    "channel_id": "<channel id>",
    "status": "executed"
  }
}
```

`id` equals the audit record's `event_id`, so a receiver can correlate the
two. `args` is the raw argument string and is omitted when empty.

---

## 4. Implementation Status

1. **AuthZ:** CUE-to-Keto reconciliation (`chit-reconcile`, `make reconcile`). Done.
2. **Logging:** JSON audit log. Done.
3. **Interceptor:** command parsing and Keto check in `POST /posts`. Done.
4. **Webhook:** async dispatcher with a worker pool and HMAC signing. Done.
5. **API:** `GET /api/v1/commands` for client-side discovery, returning
   `[{id, slug, description, category}]` (`[]` when commands are disabled).
   Done.
6. **`/summarize` handler.** Not done.

---

## 5. Out of Scope

* **TUI UX:** Client-side rendering and autocomplete UI are excluded.
* **Persistence:** Slash commands are not persisted to the chat tables.
