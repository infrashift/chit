---
id: "01a1f537-9e6c-7e21-98c9-9336dbd4289d"
title: "ISSUE-009: R5: assertion density"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-9"
  - "power-of-10"
  - "r5"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Add internal/assert to each module and reach an average of two assertions for each function."
---

# ISSUE-009: R5: assertion density

**Status/Phase:** Open · Phase 9

## Description
Add internal/assert to each module and reach an average of two assertions for each function.

## Acceptance Criteria
- [ ] Each module has `internal/assert` with tests.
- [ ] semscope reports an average of 2 or more assertions for each function in each module.
- [ ] `forbidigo` bans `panic` outside `internal/assert`.
- [ ] Worker, pump and listener goroutines have a recover guard.
- [ ] NASA-P10-05 is not omitted and the omission list is empty.
- [ ] `make check`, `make test-integration` and `make test-e2e` pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
