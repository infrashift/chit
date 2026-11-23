---
x-okfw-id: "01a1f5dd-dc7e-7f29-9948-59f1ccc44783"
title: "ADR-0013: A membership policy hook decides each channel add"
type: "ADR"
tags:
  - "adr"
  - "membership"
  - "security"
x-okfw-created: "2026-11-19T16:12:00-06:00"
x-okfw-timestamp: "2026-11-19T16:12:00-06:00"
description: "When CHIT_MEMBERSHIP_HOOK_URL is set, chitd asks the hook before each channel add, and only an answer of allow lets the add occur."
---

# ADR-0013: A membership policy hook decides each channel add

**Status:** Accepted
**Date:** 2026-11-19 (records the decision of PR #9)
**Source:** HALO prerequisite P-CHIT-2. `CHANGELOG.md`.

## Context
HALO must decide which person can be in which Conversation. chitd checks only the team and the channel rules.

## Decision
- When the operator sets `CHIT_MEMBERSHIP_HOOK_URL`, `internal/memberhook` sends a POST to the hook before each channel add. The body names the channel, the user, the Kratos identity, the actor type, the user who adds and the client.
- The request has a Hydra `client_credentials` token of its own client.
- Only `{"allow":true}` allows the add. A refusal, another status, a timeout, a network error or bad JSON refuses it with 403 `channel.membership_refused`.
- A request that adds more than one member adds all of them or none.
- Each answer is a `membership_check` audit record.

## Consequences
- Without the URL, chitd does not change.
- The dev stack runs a stub of the hook (`tests/e2e/hookstub`) for e2e scenario 12. semscope skips the stub, and golangci-lint checks it.

## Related
- [ADR-0006](/decisions/adr-0006-the-ory-stack-authenticates-and-chitd-authorizes--01a1f5379e13.md)
