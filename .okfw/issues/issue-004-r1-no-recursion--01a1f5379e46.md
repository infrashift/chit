---
id: "01a1f537-9e46-72b1-bcd7-a54fb937cc58"
title: "ISSUE-004: R1: no recursion"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-4"
  - "power-of-10"
  - "r1"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Remove the two recursion cycles in chit-tui."
---

# ISSUE-004: R1: no recursion

**Status/Phase:** Open · Phase 4

## Description
Remove the two recursion cycles in chit-tui.

## Acceptance Criteria
- [ ] `rgbComponents` has no recursion.
- [ ] `Model.Update` does not call itself through `handleUserLoaded` and `resumeAfterReLogin`.
- [ ] `gocognit` and `nestif` are on.
- [ ] NASA-P10-01 is not omitted.
- [ ] `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
