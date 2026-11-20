---
id: "01a1f5dd-dc85-7e31-b838-9a808031f2f9"
title: "ADR-0014: Rate limits for each person and for each on-behalf-of client"
type: "ADR"
tags:
  - "adr"
  - "rate-limit"
  - "on-behalf-of"
created: "2026-11-19T16:12:00-06:00"
timestamp: "2026-11-19T16:12:00-06:00"
description: "Each person keeps a per-user limit, and an on-behalf-of client has an aggregate limit of its own, so halod can act for many people through one machine actor."
---

# ADR-0014: Rate limits for each person and for each on-behalf-of client

**Status:** Accepted
**Date:** 2026-11-19 (records the decision of PR #10)
**Source:** HALO prerequisite P-CHIT-8. `CHANGELOG.md`.

## Context
halod acts for each person through one machine actor. Its own requests shared one per-user bucket of 10 requests each second. HALO's load test was stable at 60 people of its target of 2,000.

## Decision
- A person, and a machine actor that is not an on-behalf-of client, has 10 requests each second, with bursts of 50 (`CHIT_RATE_LIMIT_USER_*`).
- An on-behalf-of client has an aggregate bucket of 1,500 requests each second, with bursts of 3,000 (`CHIT_RATE_LIMIT_CLIENT_*`). The bucket counts each request of the client.
- A request on behalf of a person also counts against the bucket of the person.
- A request takes a token from each of its buckets or from none.
- Each 429 has `Retry-After`. Each refusal writes a `rate limit exceeded` warning.

## Consequences
- chitd does not start with a rate of 0 or less, or a burst less than 1.

## Related
- [ADR-0009](/decisions/adr-0009-allowlisted-machine-actors-act-on-behalf-of-a-person--01a1f5379e28.md)
- [On-behalf-of requests](/specs/on-behalf-of-requests--01a1f5379dea.md)
