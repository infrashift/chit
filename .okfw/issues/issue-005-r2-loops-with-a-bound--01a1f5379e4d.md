---
id: "01a1f537-9e4d-7dda-9cc7-c2b4802712c3"
title: "ISSUE-005: R2: loops with a bound"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-5"
  - "power-of-10"
  - "r2"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Each for loop states its exit condition or its maximum count."
---

# ISSUE-005: R2: loops with a bound

**Status/Phase:** Open · Phase 5

## Description
Each for loop states its exit condition or its maximum count.

## Acceptance Criteria
- [ ] No `for {}` in either module.
- [ ] Long-lived loops use `for ctx.Err() == nil`.
- [ ] Retry loops have a maximum count.
- [ ] NASA-P10-02 is not omitted.
- [ ] `make check`, `make test-integration` and a shutdown smoke test pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
