---
id: "01a1f537-9e5d-74b7-bc9b-7c054a383fef"
title: "ISSUE-007: R6: no mutable package variables"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-7"
  - "power-of-10"
  - "r6"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Move or remove package variables, and suppress only the kinds that ADR-0002 allows."
---

# ISSUE-007: R6: no mutable package variables

**Status/Phase:** Open · Phase 7

## Description
Move or remove package variables, and suppress only the kinds that ADR-0002 allows.

## Acceptance Criteria
- [ ] Each package variable is gone or has a suppression with a reason.
- [ ] `gochecknoglobals` is on.
- [ ] NASA-P10-06 is not omitted.
- [ ] `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
