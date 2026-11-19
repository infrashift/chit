---
id: "01a1f537-9e65-711d-9463-cf2d41f95fb7"
title: "ISSUE-008: R4: functions of 60 lines or fewer"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-8"
  - "power-of-10"
  - "r4"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Split each function that has more than 60 lines."
---

# ISSUE-008: R4: functions of 60 lines or fewer

**Status/Phase:** Open · Phase 8

## Description
Split each function that has more than 60 lines.

## Acceptance Criteria
- [ ] No function has more than 60 lines.
- [ ] `funlen` is on with 60 lines.
- [ ] NASA-P10-04 is not omitted.
- [ ] `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
