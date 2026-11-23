---
x-okfw-id: "01a1f537-9e65-711d-9463-cf2d41f95fb7"
title: "ISSUE-008: R4: functions of 60 lines or fewer"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-8"
  - "power-of-10"
  - "r4"
  - "closed"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T14:06:34-06:00"
description: "Split each function that has more than 60 lines."
---

# ISSUE-008: R4: functions of 60 lines or fewer

**Status/Phase:** Closed · Phase 8

## Description
Split each function that has more than 60 lines.

## Acceptance Criteria
- [x] No function has more than 60 lines.
- [x] `funlen` is on with 60 lines.
- [x] `gocognit` (20) and `nestif` (4) are on (moved from Phase 4).
- [x] NASA-P10-04 is not omitted.
- [x] `make check` passes.

## Resolution
Done on 2026-11-19 in Phase 8.

- No function has more than 60 lines. semscope NASA-P10-04 reported 27 findings before. Now it reports zero, and `.semscope.cue` does not omit it.
- `funlen` (60), `gocognit` (20) and `nestif` (4) are on in both modules. They reported 67 issues before. Now they report zero.
- Server: `api.New` uses route-group helpers. `mcp.registerTools` uses one helper for each tool group. `AuthExtract`, `CreatePostAs`, `SearchPosts`, `LoadCUE` and the bridge handlers use named steps.
- TUI: `Update` checks the window size, the login state and an expired session. Then it sends the message to five small routers: session, channel, post, tag and lookup. The handlers are next to the handlers that they resemble.
- TUI: `StyleConfig` and `styles.New` use section helpers. A temporary test compared the old and new styles for all 8 built-in themes. They were the same.
- Test tables moved into case functions. The tests keep each case and each assertion.
- `make check`, `make test-integration` and `make test-keto` pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
