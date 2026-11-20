---
id: "01a1f771-12be-7242-b45c-4755ad21d835"
title: "ISSUE-019: The TUI has no reusable Bubbles for confirm, chips, prompt, activity and file pick"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-13"
  - "chit-tui"
  - "open"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-19T23:32:25-06:00"
description: "The TUI keeps a confirmation, a chip row and other small UI parts inside larger models, so new features cannot use them again."
---

# ISSUE-019: The TUI has no reusable Bubbles for confirm, chips, prompt, activity and file pick

**Status/Phase:** Open · Phase 13

## Description
The root model has its own y/N prompt before a delete. The member picker draws its own chips. The HALO features need a y/N prompt, chips, a one-line prompt, a set of short-lived indicators and a file picker. Each part must be a Bubble in internal/tui/ui that other parts can use again.

## Acceptance Criteria
- [ ] The packages `ui/confirm`, `ui/chips`, `ui/prompt`, `ui/activity` and `ui/filepick` exist, each with tests.
- [ ] A Bubble imports no tui, api or model package. It has no function fields. It reports with tea.Msg types.
- [ ] The post delete uses the `ui/confirm` package. The member picker uses `ui/chips`. The behaviour does not change.
- [ ] An ADR records the contract of a Bubble.
- [ ] make check passes.
