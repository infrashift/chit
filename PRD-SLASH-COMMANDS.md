For **Chit** to serve as a robust foundation for your AI-augmented SDLC, the webhook system must be more than an afterthought—it needs to be a high-performance, standardized "Event Stream."

Integrating **CloudEvents (CNCF)** as the envelope standard ensures that any downstream bot or service (even those outside the Go/Ory ecosystem) can parse Chit events without custom logic.

---

## PRD-SLASH-COMMANDS.md (Updated)

**Project:** Chit

**Feature:** Sovereign Command Intercept, Audit, & Eventing

**Status:** Implemented, except where noted below. See
[SPECS-SLASH-COMMANDS.md](SPECS-SLASH-COMMANDS.md) for the behaviour as built.

### 1. Objective

Establish a deterministic, role-based command system for the Chit platform. This system must treat Humans, AI Agents, and Bots as identical "Actors" subject to a centralized, CUE-defined permission hierarchy, providing real-time auditability and external extensibility via standardized webhooks.

### 2. Functional Requirements

* **Universal Intercept:** Every message entry point must check for the `/` prefix. *(Implemented: `POST /api/v1/posts` is the only entry point that creates messages, and it intercepts commands after checking channel membership.)*
* **Role-Based Access (ReBAC):** Access must resolve via: `Actor -> Role -> Command`.
* **Hermetic Configuration:** All AuthZ metadata must be managed in CUE.
* **Structured Audit Logging:** Every command attempt (Success/Failure) must be logged to a local file using `slog` in JSON format. *(Implemented for every registered command that reaches the permission check, allowed or denied. Unknown commands are not logged.)*
* **Outbound Webhooks (Opt-in):** **(NEW)** When configured, the platform must emit a **CloudEvents v1.0** compliant JSON payload to a registered URL upon successful command execution. *(Implemented for commands that change state: `/invite`, `/kick`, `/topic <text>`. Delivery is best-effort with no retry.)*

### 3. Success Criteria

* **Interoperability:** Webhook payloads pass validation against the CloudEvents JSON schema.
* **Admin Sovereignty:** Webhooks are disabled by default and require explicit administrator "Opt-in" per channel or global scope. *(Global opt-in only, via `CHIT_WEBHOOK_ENABLED`. Per-channel opt-in is **not implemented**.)*

