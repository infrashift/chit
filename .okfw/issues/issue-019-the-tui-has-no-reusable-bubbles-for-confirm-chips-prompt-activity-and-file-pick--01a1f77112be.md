---
x-okfw-id: "01a1f771-12be-7242-b45c-4755ad21d835"
title: "ISSUE-019: The TUI has no reusable Bubbles for confirm, chips, prompt, activity and file pick"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-13"
  - "chit-tui"
  - "closed"
x-okfw-created: "2026-11-19T23:32:25-06:00"
x-okfw-timestamp: "2026-11-19T23:55:01-06:00"
description: "The TUI keeps a confirmation, a chip row and other small UI parts inside larger models, so new features cannot use them again."
---

# ISSUE-019: The TUI has no reusable Bubbles for confirm, chips, prompt, activity and file pick

**Status/Phase:** Closed · Phase 13

## Description
The root model has its own y/N prompt before a delete. The member picker draws its own chips. The HALO features need a y/N prompt, chips, a one-line prompt, a set of short-lived indicators and a file picker. Each part must be a Bubble in internal/tui/ui that other parts can use again.

## Acceptance Criteria
- [x] The packages `ui/confirm`, `ui/chips`, `ui/prompt`, `ui/activity` and `ui/filepick` exist, each with tests.
- [x] A Bubble imports no tui, api or model package. It has no function fields. It reports with tea.Msg types.
- [x] The post delete uses the `ui/confirm` package. The member picker uses `ui/chips`. The behaviour does not change.
- [x] An ADR records the contract of a Bubble.
- [x] make check passes.

## Resolution
Done on 2026-11-19.

- Five Bubbles are in `internal/tui/ui`: `confirm`, `chips`, `prompt`, `activity` and `filepick`. Their tests cover 96% to 100% of their statements.
- The post delete asks through `ui/confirm`. The member picker keeps its picks in `ui/chips`. The old tests pass with no change, and two new `teatest` flows drive both through a real program.
- `routeBubbleMsg` in `bubbles.go` routes the messages of the Bubbles. An error closes an open question.
- [ADR-0015](/decisions/adr-0015-reusable-bubbles-in-internal-tui-ui--01a1f782e7a8.md) records the contract of a Bubble.
