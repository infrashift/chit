---
id: "01a1f537-9e1a-73a7-9466-17105032e02a"
title: "ADR-0007: Thin pub/sub envelopes with PG LISTEN/NOTIFY by default"
type: "ADR"
tags:
  - "adr"
  - "pubsub"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "chitd publishes thin event envelopes on chit_events through PG LISTEN/NOTIFY or NATS, and WebSocket fan-out stays in-process."
---

# ADR-0007: Thin pub/sub envelopes with PG LISTEN/NOTIFY by default

**Status:** Accepted
**Date:** 2026-11-19 (records an earlier decision)
**Source:** `docs/src/content/docs/architecture/pubsub.mdx`.

## Context
Fan-out to more than one node needs a message bus. PostgreSQL is already there. A `NOTIFY` payload has a limit of about 8 KB.

## Decision
- `internal/pubsub` has one interface with two backends: `pgnotify` (the default) and `nats`. `CHIT_PUBSUB_BACKEND` selects one. chitd refuses other values at startup.
- chitd publishes a thin envelope on `chit_events`: the event type and IDs, not the full payload.
- A topic name must be a valid PostgreSQL identifier.
- The pgnotify listener connects again and sends `LISTEN` again after a connection failure.
- WebSocket fan-out stays in-process. No component subscribes to `chit_events` yet.

## Consequences
- chitd runs on one node until a component reads `chit_events`.
- The R2 adaptation of [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md) applies to the listener loop.

## Related
- [WebSocket events](/specs/websocket-events--01a1f5379de0.md)
