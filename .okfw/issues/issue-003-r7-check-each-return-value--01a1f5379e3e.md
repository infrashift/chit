---
id: "01a1f537-9e3e-7986-9c81-1afde5cd6aa3"
title: "ISSUE-003: R7: check each return value"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-3"
  - "power-of-10"
  - "r7"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "No discarded call result and no dropped error in either Go module."
---

# ISSUE-003: R7: check each return value

**Status/Phase:** Open · Phase 3

## Description
No discarded call result and no dropped error in either Go module.

## Acceptance Criteria
- [ ] semscope NASA-P10-07 with `go_types` reports zero findings.
- [ ] `errcheck` has `check-blank` and `check-type-assertions`. `errorlint` is on.
- [ ] The omission of NASA-P10-07 is gone from `.semscope.cue`.
- [ ] `make check` and `make test-integration` pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
