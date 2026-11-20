---
id: "01a1f537-9e55-791a-b860-2ac8cc46c460"
title: "ISSUE-006: R9: no function types in chit code"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-6"
  - "power-of-10"
  - "r9"
  - "closed"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:46:06-06:00"
description: "Replace function-typed fields and dispatch tables with interfaces or switch statements."
---

# ISSUE-006: R9: no function types in chit code

**Status/Phase:** Closed · Phase 6

## Description
Replace function-typed fields and dispatch tables with interfaces or switch statements.

## Acceptance Criteria
- [x] Fields of function type in chit code are gone.
- [x] Each function type that a framework requires has a suppression with a reason.
- [x] NASA-P10-09 is not omitted.
- [x] `make check` passes.

## Resolution
Done on 2026-11-19 in Phase 6.

- Server: `api` and `workers` use a `clock` interface. `chitclient.Client.Listen` takes an `EventHandler`. `pubsub.Subscribe` takes a `pubsub.Handler`. `showProgress` returns a `progressNote` with a `Stop` method.
- Server: `command.HandlerFunc` is gone. `/help`, `/invite`, `/kick` and `/topic` are small types in `internal/server/commands.go`. A new test covers the help text.
- Server: `AuthExtract`, `RateLimit` and `NewValidationMiddleware` keep `func(http.Handler) http.Handler`, because chi `Router.Use` requires it. Each has a suppression with that reason.
- TUI: `storedToken` takes two small interfaces. `api.NewClientWithTokenSource` takes an `api.TokenSource`. `theme.Request.System` is a `theme.DarkDetector`, and the query to the terminal stays lazy.
- TUI: the overlay table holds an overlay ID and a pointer to each overlay. A `switch` in `updateOverlay` sends the keys. A new test checks that each ID reaches its own overlay.
- TUI: the config keys and the built-in themes use an ordered list and a `switch`. The ws client keeps the backoff limits as fields.
- semscope skips `internal/store/storetest` and `clients/chit-tui/internal/testutil`. Only tests import them.
- semscope NASA-P10-09 reports zero findings, and `.semscope.cue` does not omit it.
- `make check` and `make test-integration` pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
