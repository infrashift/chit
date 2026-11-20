---
id: "01a1f56c-dcd3-7df3-b7a6-7c755f6e18ad"
title: "ADR-0011: A failed assertion stops one unit of work or the process"
type: "ADR"
tags:
  - "adr"
  - "power-of-10"
  - "r5"
  - "assertions"
created: "2026-11-19T14:08:34-06:00"
timestamp: "2026-11-19T14:08:34-06:00"
description: "assert.That panics; a request gives a 500, a guarded goroutine for one connection or thread stops alone, and a loop that the process needs stops the process so that Podman restarts it."
---

# ADR-0011: A failed assertion stops one unit of work or the process

**Status:** Accepted
**Date:** 2026-11-19
**Source:** [ISSUE-009](/issues/issue-009-r5-assertion-density--01a1f5379e6c.md). [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md) requires an average of two assertions for each function.

## Context
`assert.That` panics when its condition is false. A panic in a goroutine without a `recover` stops the process. A `recover` that only logs can leave the process alive but broken, for example a hub without its event loop. The pods in `deploy/chit.kube.yml` and `deploy/chit-uat.kube.yml` have `restartPolicy: Always`.

## Decision
- Each Go module has `internal/assert` with one function, `That(cond, msg)`. A failed assertion panics with `chit: assertion failed: <msg>`.
- An assertion checks a programmer error only. Bad input from a request, the database, a file or the terminal returns an error, as before.
- **HTTP requests:** chi `middleware.Recoverer` changes a panic into a 500 response.
- **One unit of work:** a goroutine for one connection, one thread or one background call defers `errlog.Recover` as its first statement. It logs the panic with the stack, and only that goroutine stops. These goroutines use it: the WebSocket read and write pumps, the bridge replies, the Keto mirror call, and the TUI WebSocket session and ping. The bridge thread worker uses `recoverThread`, which also removes the state of the thread.
- **A loop that the process needs** has no `recover`. This group has the hub, the scheduler, the webhook pool, pgnotify, the search indexer and the MCP event feed. A failed assertion stops the process, and Podman starts it again with a clean state. The membership load of the hub is in this group, because a lost load blocks the events of one user until a restart.
- **Bubble Tea** stops a panic in `Update` and `View` and restores the terminal.
- `forbidigo` bans `panic` outside `internal/assert` and the test files.

## Consequences
- A defect in a request or a connection does not stop the other users.
- A defect in a central loop stops the process, and the operator sees the restart.
- [ADR-0010](/decisions/adr-0010-errlog-checks-the-errors-that-a-caller-cannot-return--01a1f53e0eaa.md) gives `errlog`, which now also holds `Recover`.

## Alternatives considered
- **Recover in every goroutine.** Rejected: a central loop that stops quietly leaves the process alive but broken.
- **No recover at all.** Rejected: one bad connection then stops chitd for all users.
