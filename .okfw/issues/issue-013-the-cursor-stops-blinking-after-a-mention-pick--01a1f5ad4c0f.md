---
x-okfw-id: "01a1f5ad-4c0f-7b90-85fc-96fbde81486f"
title: "ISSUE-013: The input cursor of chit-tui never blinks"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "bug"
  - "tui"
  - "closed"
x-okfw-created: "2026-11-19T15:18:57-06:00"
x-okfw-timestamp: "2026-11-19T15:58:02-06:00"
description: "The root model of chit-tui drops each cursor blink message, so the input cursor never blinks; the blink commands of the input are dead code."
---

# ISSUE-013: The input cursor of chit-tui never blinks

**Status/Phase:** Closed · Bug

## Description
The Phase 3 agent reported that a mention pick drops the blink command of its backspaces. A probe on 2026-11-19 found a wider fact. The root `Update` of `clients/chit-tui/internal/tui` gives no message to the input unless a router claims it. No router claims `cursor.BlinkMsg`, so the root drops each blink. This was also true before the refactor.

- The textarea uses the blink mode, and each key returns a blink command. The first blink arrives, and the root drops it.
- The cursor never blinks. It stays a steady, visible block after each key.
- A mention pick does not change this. The cursor does not stop, because it never started.

## Options
1. **Make the cursor blink.** The root gives `cursor.BlinkMsg` to the focused editor. `ReplaceAtMention` returns its blink command.
2. **Make the cursor steady by design.** The cursor uses the static mode. The input returns no blink commands.

## Acceptance Criteria
- [x] The user chooses an option. (Option 1: make the cursor blink.)
- [x] A test shows the chosen behavior through the root model.

## Resolution
Done on 2026-11-19. The user chose option 1: make the cursor blink.

- `routeBlink` in the root `Update` gives each `cursor.BlinkMsg` to five editors. They are the input, the palette, the two pickers and the channel creator. A cursor answers only a blink with its own ID, and an editor without focus ignores it.
- A blink is no edit. The input gives a blink to the textarea only, so it does not send the mention trigger again. The channel creator gives a blink to the active field only, so it keeps the validation error.
- `ReplaceAtMention` returns the blink of its backspaces, and the mention handler returns it.
- The first user of a session starts the blink, which `NewModel` cannot run.
- The sign-in screen already gave each message to its fields, so its cursor blinked before this change.
- New tests cover the blink cycle after a key, the first blink after sign-in and the blink after a mention pick. Two more tests show that a blink changes no state. A temporary change showed that the mention-trigger test fails without the fix.
- `make check` passes. The TUI tests pass twice with `-race`.

## Related
- [ISSUE-003](/issues/issue-003-r7-check-each-return-value--01a1f5379e3e.md)
