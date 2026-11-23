---
x-okfw-id: "01a1f537-9e13-728d-915b-47322ab67de8"
title: "ADR-0006: The Ory stack authenticates and chitd authorizes"
type: "ADR"
tags:
  - "adr"
  - "auth"
  - "security"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T13:10:25-06:00"
description: "Oathkeeper, Kratos and Hydra authenticate each request; chitd checks permissions against PostgreSQL membership, and Keto authorizes slash commands only."
---

# ADR-0006: The Ory stack authenticates and chitd authorizes

**Status:** Accepted
**Date:** 2026-11-19 (records an earlier decision)
**Source:** `README.md`, `ROADMAP.md` phase 3, `CHANGELOG.md`.

## Context
chit has people and machine actors. People log in with a browser session. Agents and bots use OAuth2 tokens. chit must not keep passwords.

## Decision
- Kratos holds the identities of people. Hydra issues `client_credentials` tokens to machine actors. Oathkeeper is the proxy in front of chitd and sets `X-User-Id` or `X-Client-Id`.
- chitd trusts these headers. With `CHIT_TRUSTED_PROXY_SECRET`, it trusts them only when `X-Proxy-Secret` matches.
- chitd makes the user record of a person on the first request. A machine actor must be in `CHIT_MACHINE_ACTORS`.
- PostgreSQL membership (`channel_members`) decides channel access. chitd mirrors the channel tuples to Keto, but does not ask Keto for channel access.
- Keto (keto-cue, with a CUE schema in `deploy/keto/schema/`) authorizes slash commands in `chit/command`.

## Consequences
- A Keto failure does not stop reads or posts.
- [On-behalf-of requests](/specs/on-behalf-of-requests--01a1f5379dea.md) adds one more header for allowlisted clients ([ADR-0009](/decisions/adr-0009-allowlisted-machine-actors-act-on-behalf-of-a-person--01a1f5379e28.md)).

## Related
- [chit architecture overview](/architecture/chit-architecture-overview--01a1f5379dd5.md)
