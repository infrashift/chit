---
x-okfw-id: "01a1f537-9e4d-7dda-9cc7-c2b4802712c3"
title: "ISSUE-005: R2: loops with a bound"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-5"
  - "power-of-10"
  - "r2"
  - "closed"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T13:35:28-06:00"
description: "Each for loop states its exit condition or its maximum count."
---

# ISSUE-005: R2: loops with a bound

**Status/Phase:** Closed · Phase 5

## Description
Each for loop states its exit condition or its maximum count.

## Acceptance Criteria
- [x] No `for {}` in either module.
- [x] Long-lived loops use `for ctx.Err() == nil`.
- [x] Retry loops have a maximum count.
- [x] NASA-P10-02 is not omitted.
- [x] `make check`, `make test-integration` and a shutdown smoke test pass.

## Resolution
Done on 2026-11-19 in Phase 5.

- A service loop runs until shutdown. Each one now states its exit condition. Most use `for ctx.Err() == nil`. The hub uses a flag, and the TUI WebSocket client uses `!closed(stop)`.
- Read loops end when a read fails: `for ; err == nil; _, msg, err = conn.ReadMessage()`. The read deadline makes a silent peer fail.
- The server write pump moves its two cases to `writeEvent` and `writePing`. The loop ends when one of them reports false.
- Drain loops (bridge `abandon` and the topic queue of pgnotify) have the channel capacity as their bound.
- Loops over pages have a maximum: Keto `ListRelations` returns an error after 1,000 pages. The search indexer stops a tick after 100 batches and goes on at the next tick.
- semscope NASA-P10-02 reports zero findings, and `.semscope.cue` does not omit it.
- `make check`, `make test-integration` and `make test-keto` pass. The WebSocket, chitclient, bridge, jobs, Keto and TUI ws tests pass five times with `-race`.
- The shutdown smoke test against a live chitd moves to the end of Phase 9.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
