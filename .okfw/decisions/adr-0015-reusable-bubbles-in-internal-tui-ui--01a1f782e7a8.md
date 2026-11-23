---
x-okfw-id: "01a1f782-e7a8-7e44-ada3-1d4d3f136d00"
title: "ADR-0015: Reusable Bubbles in internal/tui/ui"
type: "ADR"
tags:
  - "adr"
  - "chit-tui"
  - "bubbles"
x-okfw-created: "2026-11-19T23:51:53-06:00"
x-okfw-timestamp: "2026-11-19T23:51:53-06:00"
description: "Each small UI part that more than one feature needs is a Bubble in internal/tui/ui, with a fixed contract: no chit imports, no function fields, messages for its output, SetStyles, and its own tests."
---

# ADR-0015: Reusable Bubbles in internal/tui/ui

**Status:** Accepted
**Date:** 2026-11-19
**Source:** [ISSUE-019](/issues/issue-019-the-tui-has-no-reusable-bubbles-for-confirm-chips-prompt-activity-and-file-pick--01a1f77112be.md). The user asked for reusable Bubbles where they fit.

## Context
The root model had its own y/N question for a post delete. The member picker drew its own chips. The HALO features need a y/N question, chips, a one-line prompt, a set of short-lived indicators and a file picker. A copy of each part in each feature makes the code larger, and the copies drift apart.

## Decision
Each small UI part that two or more features need is a Bubble in its own package under `clients/chit-tui/internal/tui/ui/`. The first five are `ui/confirm`, `ui/chips`, `ui/prompt`, `ui/activity` and `ui/filepick`.

A Bubble obeys this contract:
- It imports only Bubble Tea, bubbles, lipgloss, the x/ansi package, `internal/assert` and `ui/styles`. It does not import `tui`, `api` or `model`, so other clients can use it.
- It has no fields of function type ([ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md) R9). It sends a result as an exported `tea.Msg` type. When the parent must act in the same `Update`, it returns the result.
- It has `SetStyles`, so a theme change reaches it.
- Each collection in it has a cap, and each timer carries a sequence number, so an old timer does not close a newer item.
- Its own tests cover it.

The root model routes the messages of the Bubbles in one router, `routeBubbleMsg` in `bubbles.go`.

## Consequences
- `ui/confirm` returns its answer from `Answer`, not in a message. The tests of the root model run a command but do not feed its messages back into `Update`. A message also adds one frame between the key and the delete.
- An error closes an open question (`setError`). The status line shows one thing, and a key must not answer a question that is not on screen.
- `ui/filepick` wraps the file picker of bubbles. It adds the module `github.com/dustin/go-humanize` to `go.sum`, as a dependency of bubbles.

## Related
- [ADR-0008](/decisions/adr-0008-chit-tui-is-a-separate-go-module--01a1f5379e21.md)
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
