---
x-okfw-id: "01a1f537-9e46-72b1-bcd7-a54fb937cc58"
title: "ISSUE-004: R1: no recursion"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-4"
  - "power-of-10"
  - "r1"
  - "closed"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T13:30:55-06:00"
description: "Remove the two recursion cycles in chit-tui."
---

# ISSUE-004: R1: no recursion

**Status/Phase:** Closed · Phase 4

## Description
Remove the two recursion cycles in chit-tui.

## Acceptance Criteria
- [x] `rgbComponents` has no recursion.
- [x] `Model.Update` does not call itself through `handleUserLoaded` and `resumeAfterReLogin`.
- [x] `gocognit` and `nestif` move to Phase 8, because they report the same long functions as R4.
- [x] NASA-P10-01 is not omitted.
- [x] `make check` passes.

## Resolution
Done on 2026-11-19 in Phase 4.

- `rgbComponents` parses a `lipgloss.Color` before the type switch, so it does not call itself.
- `handleUserLoaded` and `resumeAfterReLogin` call the new `applyLoadedUser`. `resumeAfterReLogin` does not call `Update`, so the type assertion and its panic are gone.
- semscope NASA-P10-01 reports zero findings, and `.semscope.cue` does not omit it.
- `gocognit` and `nestif` move to Phase 8. With the thresholds of semscope (20 and 4), they report the same long functions as R4.
- `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
