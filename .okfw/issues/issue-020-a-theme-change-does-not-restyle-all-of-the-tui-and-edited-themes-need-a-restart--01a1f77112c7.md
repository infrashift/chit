---
id: "01a1f771-12c7-7e98-b925-51571bb65ef9"
title: "ISSUE-020: A theme change does not restyle all of the TUI, and edited themes need a restart"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-14"
  - "chit-tui"
  - "open"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-19T23:32:25-06:00"
description: "After /theme some parts keep the old colors, an edited theme file needs a restart, and the theme picker has no preview."
---

# ISSUE-020: A theme change does not restyle all of the TUI, and edited themes need a restart

**Status/Phase:** Open · Phase 14

## Description
The user reports that some parts of the screen keep the old colors after /theme. A change to a theme file has no effect until the TUI starts again. The picker applies a theme only on Enter, and /theme accepts no name. Few users know about /theme.

## Acceptance Criteria
- [ ] One restyle path reaches each component.
- [ ] A test shows that a theme change gives the same screen as a fresh start with that theme.
- [ ] The picker shows a preview when the cursor moves. Esc goes back to the old theme.
- [ ] /theme <name> and /theme reload work. A watch reloads an edited local theme file.
- [ ] The help overlay and the docs tell how to change the theme.
- [ ] make check passes.
