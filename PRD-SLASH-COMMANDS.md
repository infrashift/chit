**Chit** must be a strong base for an AI-augmented SDLC. For this, the webhook system cannot be an afterthought. It must be a fast, standard "Event Stream."

**CloudEvents (CNCF)** is the envelope standard. So any downstream bot or service can parse Chit events without custom logic, also outside the Go/Ory ecosystem.

---

## PRD-SLASH-COMMANDS.md (Updated)

**Project:** Chit

**Feature:** Sovereign Command Intercept, Audit, & Events

**Status:** Implemented, except where noted below. See
[SPECS-SLASH-COMMANDS.md](SPECS-SLASH-COMMANDS.md) for the behaviour as built.

### 1. Objective

Establish a deterministic, role-based command system for the Chit platform. This system must treat Humans, AI Agents, and Bots as identical "Actors" under one central permission hierarchy that CUE defines. The system must give real-time audit records, and others must be able to extend it through standard webhooks.

### 2. Functional Requirements

* **Universal Intercept:** Every message entry point must check for the `/` prefix. *(Implemented: `POST /api/v1/posts` is the only entry point that creates messages. It intercepts commands after it checks channel membership.)*
* **Role-Based Access (ReBAC):** Access must resolve via: `Actor -> Role -> Command`.
* **Hermetic Configuration:** CUE must hold all AuthZ metadata.
* **Structured Audit Logs:** chitd must log each try to run a command (Success/Failure) to a local file. It writes the log with `slog` in JSON format. *(Implemented for every registered command that reaches the permission check, allowed or denied. chitd does not log unknown commands.)*
* **Outbound Webhooks (Opt-in):** **(NEW)** An operator can configure webhooks. Then, after a command runs successfully, the platform must emit a **CloudEvents v1.0** compliant JSON payload to a registered URL. *(Implemented for commands that change state: `/invite`, `/kick`, `/topic <text>`. Delivery is best-effort with no retry.)*

### 3. Success Criteria

* **Interoperability:** Webhook payloads pass validation against the CloudEvents JSON schema.
* **Admin Sovereignty:** Webhooks are off by default. An administrator must turn them on explicitly ("Opt-in"), per channel or for the global scope. *(Global opt-in only, via `CHIT_WEBHOOK_ENABLED`. Per-channel opt-in is **not implemented**.)*
