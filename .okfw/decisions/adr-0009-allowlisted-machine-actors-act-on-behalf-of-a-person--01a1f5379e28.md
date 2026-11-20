---
id: "01a1f537-9e28-7607-bf6b-24ecc8101b13"
title: "ADR-0009: Allowlisted machine actors act on behalf of a person"
type: "ADR"
tags:
  - "adr"
  - "auth"
  - "on-behalf-of"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "X-On-Behalf-Of lets a client in CHIT_ON_BEHALF_OF_CLIENTS act as a person on an allowlisted route, and provisions a Kratos identity that chit has not seen."
---

# ADR-0009: Allowlisted machine actors act on behalf of a person

**Status:** Accepted
**Date:** 2026-11-19 (records the decision of commit `acfe1de`)
**Source:** HALO prerequisite P-CHIT-8. Commits `acfe1de`, `fd92648`.

## Context
halod must make a Conversation for a person before that person opens chit. A machine actor posts as itself, so it cannot do this without a new mechanism.

## Decision
- A new header, `X-On-Behalf-Of`, names the person by user ID or by `kratos:<id>`.
- Only a client in `CHIT_ON_BEHALF_OF_CLIENTS` can send it, and only on the routes of [On-behalf-of requests](/specs/on-behalf-of-requests--01a1f5379dea.md).
- For `kratos:<id>`, chitd provisions the person on the browser path and writes an audit record.
- An identity without a `username` trait gets a derived username.

## Consequences
- A new route needs an entry in the allowlist, in `internal/api/middleware.go` and in `api/openapi.yaml`.
- The e2e scenario 10 tests the flow through Oathkeeper.

## Alternatives considered
- **A token exchange in Hydra.** Not chosen: chit then needs a token for each person.
