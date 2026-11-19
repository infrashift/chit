---
id: "01a1f537-9e55-791a-b860-2ac8cc46c460"
title: "ISSUE-006: R9: no function types in chit code"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-6"
  - "power-of-10"
  - "r9"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Replace function-typed fields and dispatch tables with interfaces or switch statements."
---

# ISSUE-006: R9: no function types in chit code

**Status/Phase:** Open · Phase 6

## Description
Replace function-typed fields and dispatch tables with interfaces or switch statements.

## Acceptance Criteria
- [ ] Fields of function type in chit code are gone.
- [ ] Each function type that a framework requires has a suppression with a reason.
- [ ] NASA-P10-09 is not omitted.
- [ ] `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
