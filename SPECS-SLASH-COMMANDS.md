# SPECS-SLASH-COMMANDS.md

> **Status:** implemented. This spec describes the code as built
> (`internal/command`, `internal/app/command*.go`, `auth/`). Where the
> original design was different, the code wins, and this spec tells the
> difference.

## 1. Data Architecture (CUE)

CUE is the single source of truth for command definitions and for which
roles may run them. Every file under `auth/` belongs to one CUE package,
`auth`. chitd and `chit-reconcile` both load the whole tree
(`CHIT_COMMANDS_CUE_DIR`, default `auth`) and check it before use. If the
CUE fails to load, chitd starts with slash commands off.

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

The slug has **no slash at the start**: chitd looks up `/invite` as `invite`.

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
bots have the same type of binding. The actor type lives on the user row
(`users.actor_type`), not here. By default, the CUE has no actor bindings
(`auth/actors/README.cue` is a commented example). Until an operator adds
bindings, chitd denies every command.

---

## 2. Authorization & Intercept Logic

### 2.1 The Interceptor

The app layer intercepts commands when it creates a post. So this occurs
**only through `POST /api/v1/posts`**. The WebSocket accepts no posts. In order:

1. **Membership:** first, the caller must be a member of the post's channel
   (`403` otherwise). So no one can run a command in a channel that they
   cannot read.
2. **Parse:** content that matches `^/([a-z0-9-]+)(\s+.*)?$` is a command.
   The first group is the slug. The rest (after one whitespace character) is
   the argument string. chitd posts all other content, such as
   `/usr/local/bin`, as a normal post.
3. **Lookup:** an unknown slug answers ``Unknown command `/x`. Type `/help`
   for available commands.``
4. **AuthZ:** query Ory Keto (`CHIT_KETO_READ_URL`):
   * **Namespace:** `chit/command`
   * **Object:** `Command:<CommandID>`
   * **Relation:** `execute`
   * **Subject:** `Actor:<chit users.id>`

   A Keto error (also an error status) denies the command. The answer is
   ``Permission check failed for `/x`.``. A denial answers
   ``You do not have permission to use `/x`.``. chitd audits both.
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

The CUE is the source of truth for this namespace. The reconciler writes
every declared tuple. Then it deletes every other tuple in `chit/command`.
So, when an operator removes a grant, role or binding from the CUE, the next
run revokes it. The reconciler lists the tuples through the Keto read API
(`CHIT_KETO_READ_URL`). The list stops after 1,000 pages of 500 tuples, and a
longer list is a failure. If the list fails, the reconciler deletes nothing.

### 2.3 Handlers

| Command | Behaviour |
|---|---|
| `/help` | Lists every registered command and its description. |
| `/invite <user>` | Adds the user (first word, leading `@` optional) to the channel with the same rules as `POST /channels/{id}/members`. |
| `/kick <user>` | Removes the user with the same rules as `DELETE /channels/{id}/members/{user_id}`: to remove a user other than the caller, the caller also needs `system_admin`. |
| `/topic [text]` | With text, sets the channel header as `PUT /channels/{id}` would; without, reports the current header. |
| `/summarize` | Not implemented (no handler). |

chitd returns handler failures (unknown user, refused change) as response text.

---

## 3. Audit Log & Events

### 3.1 Structured Logging (`slog`)

A dedicated security log at `CHIT_AUDIT_LOG_PATH` (default
`/var/log/chit/audit.log`). An empty value or `-` writes to stderr. If chitd
cannot open the path, it also writes to stderr.

* **Format:** JSON lines (`slog.JSONHandler`), message `command_attempt`.
* **Fields:** `time`, `level`, `msg`, `event_id`, `actor_id`, `actor_type`,
  `command_slug`, `authorized` (bool).

chitd logs every command that reaches the permission check, allowed or not.
It does not log unknown commands.

### 3.2 Event Webhook (CloudEvents)

`CHIT_WEBHOOK_ENABLED=true` turns on webhooks globally. Then
`CHIT_WEBHOOK_URL` is mandatory. chitd queues a webhook only when a handler
reports a state change: a successful `/invite`, `/kick` or `/topic <text>`.
`/help`, `/topic` without text, and failures send nothing.

* **Request:** `POST` to `CHIT_WEBHOOK_URL`, `Content-Type:
  application/cloudevents+json` (structured mode).
* **Headers:** `ce-specversion`, `ce-type`, `ce-source`, `ce-id`.
* **Signature:** when the operator sets `CHIT_WEBHOOK_SECRET`, `X-Chit-Signature` is the
  lowercase hex HMAC-SHA256 of the exact request body, keyed by the secret
  (no prefix).
* **Delivery:** `CHIT_WEBHOOK_WORKER_COUNT` workers (default 4) read a queue of
  `CHIT_WEBHOOK_QUEUE_SIZE` (default 1024). Each request times out after
  `CHIT_WEBHOOK_TIMEOUT_SEC` (default 10). Best-effort: a full queue drops the
  event. chitd logs a failed or non-2xx delivery and does **not retry** it.

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
two. `args` is the raw argument string. chitd omits it when it is empty.

---

## 4. Implementation Status

1. **AuthZ:** reconcile CUE into Keto (`chit-reconcile`, `make reconcile`). Done.
2. **Logging:** JSON audit log. Done.
3. **Interceptor:** command parser and Keto check in `POST /posts`. Done.
4. **Webhook:** async dispatcher with a worker pool and HMAC signatures. Done.
5. **API:** `GET /api/v1/commands` for client-side discovery. It returns
   `[{id, slug, description, category}]` (`[]` when commands are off).
   Done.
6. **`/summarize` handler.** Not done.

---

## 5. Out of Scope

* **TUI UX:** This spec does not include client-side rendering and autocomplete UI.
* **Storage:** chitd does not store slash commands in the chat tables.
