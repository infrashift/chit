---
id: "01a1f5ad-4c0f-7b90-85fc-96fbde81486f"
title: "ISSUE-013: The cursor stops blinking after a mention pick"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "in-progress"
  - "bug"
  - "tui"
created: "2026-11-19T15:18:57-06:00"
timestamp: "2026-11-19T15:18:57-06:00"
description: "The root model of chit-tui drops each cursor blink message, so the input cursor never blinks; the blink commands of the input are dead code."
---

# ISSUE-013: The cursor stops blinking after a mention pick

**Status/Phase:** In progress · Bug · Waits for a decision

## Description
The Phase 3 agent reported that a mention pick drops the blink command of its backspaces. A probe on 2026-11-19 found a wider fact. The root `Update` of `clients/chit-tui/internal/tui` gives no message to the input unless a router claims it. No router claims `cursor.BlinkMsg`, so the root drops each blink. This was also true before the refactor.

- The textarea uses the blink mode, and each key returns a blink command. The first blink arrives, and the root drops it.
- The cursor never blinks. It stays a steady, visible block after each key.
- A mention pick does not change this. The cursor does not stop, because it never started.

## Options
1. **Make the cursor blink.** The root gives `cursor.BlinkMsg` to the focused editor. `ReplaceAtMention` returns its blink command.
2. **Make the cursor steady by design.** The cursor uses the static mode. The input returns no blink commands.

## Acceptance Criteria
- [ ] The user chooses an option.
- [ ] A test shows the chosen behavior through the root model.

## Related
- [ISSUE-003](/issues/issue-003-r7-check-each-return-value--01a1f5379e3e.md)
