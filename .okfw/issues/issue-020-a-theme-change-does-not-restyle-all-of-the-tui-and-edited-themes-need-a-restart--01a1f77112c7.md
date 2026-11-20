---
id: "01a1f771-12c7-7e98-b925-51571bb65ef9"
title: "ISSUE-020: A theme change does not restyle all of the TUI, and edited themes need a restart"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-14"
  - "chit-tui"
  - "closed"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-20T08:21:05-06:00"
description: "After /theme some parts keep the old colors, an edited theme file needs a restart, and the theme picker has no preview."
---

# ISSUE-020: A theme change does not restyle all of the TUI, and edited themes need a restart

**Status/Phase:** Closed · Phase 14

## Description
The user reports that some parts of the screen keep the old colors after /theme. A change to a theme file has no effect until the TUI starts again. The picker applies a theme only on Enter, and /theme accepts no name. Few users know about /theme.

## Acceptance Criteria
- [x] One restyle path reaches each component.
- [x] A test shows that a theme change gives the same screen as a fresh start with that theme.
- [x] The picker shows a preview when the cursor moves. Esc goes back to the old theme.
- [x] /theme <name> and /theme reload work. A watch reloads an edited local theme file.
- [x] The help overlay and the docs tell how to change the theme.
- [x] make check passes.

## Resolution
Done on 2026-11-20.

- `Model.restyle` is the one restyle path. A test holds each switched screen equal to a fresh start, in TrueColor, for pairs of bundled themes and each pane and overlay.
- The text boxes take the colors of the theme (`styles.ThemeTextInput`, `styles.ThemeTextArea`). `input.SetStyles` sets the style pointer of the bubbles text area again, so a preview restyles the input.
- With `theme_paint`, the client sets the colors of the terminal with OSC 10 and OSC 11, and it gives them back on exit.
- `Ctrl+T` and `/theme` open the picker on the active theme. A cursor move shows the theme, `Enter` keeps it, and `Esc` undoes it. `/theme <name>` and `/theme reload` work.
- With `theme_watch`, the client applies each saved change to the active local theme file.
- [ADR-0016](/decisions/adr-0016-a-theme-change-equals-a-fresh-start--01a1f9522f2b.md) records the decisions.
