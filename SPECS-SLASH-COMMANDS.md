
# SPECS-SLASH-COMMANDS.md

## 1. Data Architecture (CUE)

The system uses a hermetic configuration pattern where CUE serves as the single source of truth for all command definitions and access control relationships.

### 1.1 The Command Registry (`auth/schema/commands.cue`)

Acts as the central definition for all possible actions.

```cue
package schema

// The structural definition of a command
#Command: {
    id:          string & =~"^[a-z_]+$"      // Internal identifier
    slug:        string & =~"^/[a-z0-9-]+$" // The slash string
    description: string
    category:    "admin" | "chat" | "agent"
}

// Global Command Source of Truth
#Registry: {
    summarize: #Command & {
        id:          "summarize"
        slug:        "/summarize"
        description: "Generate a TL;DR of the current channel history"
        category:    "agent"
    }
    invite: #Command & {
        id:          "invite"
        slug:        "/invite"
        description: "Invite a user or agent to the channel"
        category:    "chat"
    }
}

```

### 1.2 The Role Schema & Definition (`auth/schema/roles.cue` & `auth/roles/*.cue`)

Roles reference the registry to maintain integrity.

```cue
package schema

#Role: {
    name:             string
    allowed_commands: [...#Command.slug]
}

```

```cue
// Example: auth/roles/agent.cue
package roles

import "infrashift/chit/auth/schema"

#AgentRole: schema.#Role & {
    name: "ai-assistant"
    allowed_commands: [
        schema.#Registry.summarize.slug,
        schema.#Registry.invite.slug,
    ]
}

```

### 1.3 The Actor Files (`auth/actors/*.cue`)

Individual files for every Human, AI Agent, or Bot.

```cue
package actors
import "infrashift/chit/auth/roles"

#Actor: {
    id:             string // Kratos Identity UUID
    type:           "human" | "agent" | "bot"
    assigned_roles: [...string] // References role names
}

```

---

## 2. Authorization & Intercept Logic

### 2.1 The Interceptor Middleware

The middleware intercepts messages at the API/WebSocket gateway before persistence.

* **Regex Check:** `^/([a-z0-9-]+)(\s+.*)?$`
* **Context Extraction:** Retrieve `ActorID` (Kratos Session) and `CommandID` (Lookup from Registry via slug).
* **AuthZ Check:** Query **Ory Keto** with the following relation:
* **Subject:** `Actor:<ActorID>`
* **Relation:** `execute`
* **Object:** `Command:<CommandID>`



### 2.2 Ory Keto Relationship Mapping (ReBAC)

The "Reconciler" must push these specific tuples to Keto:

1. **Role Membership:** `(Role:<role_name>#member@Actor:<uuid>)`
2. **Command Grant:** `(Command:<cmd_id>#execute@(Role:<role_name>#member))`

---

## 3. Audit Logging & Eventing

### 3.1 Structured Logging (`slog`)

A dedicated security log located at `/var/log/chit/audit.log`.

* **Format:** JSON (via `slog.JSONHandler`).
* **Required Fields:** `time`, `actor_id`, `actor_type`, `command_slug`, `authorized` (bool), `event_id`.

### 3.2 Event Webhook (CloudEvents)

Successful executions emit a **CloudEvents v1.0** payload.

* **Standard Headers:** `ce-specversion`, `ce-type`, `ce-source`, `ce-id`.
* **Security:** HMAC-SHA256 signature in `X-Chit-Signature` header using a shared secret from Vault.
* **Payload Example:**

```json
{
    "specversion": "1.0",
    "type":        "io.infrashift.chit.command.executed.v1",
    "source":      "/channels/general",
    "id":          "uuid-v4-event-id",
    "time":        "2026-03-03T10:00:00Z",
    "datacontenttype": "application/json",
    "data": {
        "actor_id":     "ryan-uuid",
        "command_slug": "/deploy",
        "args":         ["production", "--force"],
        "status":       "success"
    }
}

```

---

## 4. Implementation Roadmap for Claude Code

1. **Step 1 (AuthZ):** Implement CUE-to-Keto reconciliation utility.
2. **Step 2 (Logging):** Initialize `slog` with a custom file handler for JSON audit logs.
3. **Step 3 (Interceptor):** Build the Go middleware for command parsing and Keto check.
4. **Step 4 (Webhook):** Build an async **Webhook Dispatcher** with a worker pool and HMAC signing logic.
5. **Step 5 (API):** Implement `GET /v1/commands` for client-side discovery.

---

## 5. Out of Scope

* **TUI UX:** Client-side rendering and autocomplete UI are excluded.
* **Persistence:** Slash commands are not persisted to the standard PostgreSQL chat tables by default.

Would you like me to generate the **CUE validation CLI command** that Claude Code should use to verify the configuration before deployment?