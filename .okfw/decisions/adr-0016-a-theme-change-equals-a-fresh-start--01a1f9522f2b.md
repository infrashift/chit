---
id: "01a1f952-2f2b-7b66-8455-ef5180e39c0d"
title: "ADR-0016: A theme change equals a fresh start"
type: "ADR"
tags:
  - "adr"
  - "chit-tui"
  - "themes"
created: "2026-11-20T08:17:55-06:00"
timestamp: "2026-11-20T08:17:55-06:00"
description: "chit-tui changes its theme at run time through one restyle path, and a test holds each switched screen equal to a fresh start; the text boxes and the terminal's own colors follow the theme too."
---

# ADR-0016: A theme change equals a fresh start

**Status:** Accepted
**Date:** 2026-11-20
**Source:** [ISSUE-020](/issues/issue-020-a-theme-change-does-not-restyle-all-of-the-tui-and-edited-themes-need-a-restart--01a1f77112c7.md). The user reported three problems. Parts of the screen kept the old colors. An edited theme file needed a restart. The picker had no preview.

## Context
`/theme` already gave the new styles to each pane and overlay. Three parts did not follow the theme:
- The text boxes used the colors of bubbles. These colors follow the terminal, not the theme. The cursor line of the message input had a black or white background.
- The bubbles text area draws through a pointer to its focused or blurred style, and only `Focus` and `Blur` set the pointer. The root model is a value. After a copy, the pointer still points into the older copy. A style change shows only after the next `Focus` or `Blur`. A preview does not move the focus, so the input kept the old colors.
- The cells that nothing draws on show the terminal's own background. A light theme on a dark terminal stayed dark there.

## Decision
- **One restyle path.** `Model.restyle` gives the styles to each part. A pick, a preview, an undo, `/theme <name>`, `/theme reload` and the file watch all use it. A new part with `SetStyles` goes into it ([ADR-0015](/decisions/adr-0015-reusable-bubbles-in-internal-tui-ui--01a1f782e7a8.md)).
- **A test holds the rule.** The test uses TrueColor, pairs of bundled themes, and each pane and overlay. The screen after a switch must equal a fresh start with the new theme, byte for byte. A second test switches with the part already open, and it looks for colors that only the old theme uses.
- **The text boxes follow the theme.** `styles.ThemeTextInput` and `styles.ThemeTextArea` color each text box. `input.SetStyles` sets the style pointer again, and it returns the command of `Focus` so that the cursor continues to blink.
- **The terminal follows the theme.** With `theme_paint` (on by default), the client sets the default colors of the terminal with OSC 10 and OSC 11. It writes them straight to the terminal, as the clipboard does with OSC 52. `main` sets the colors back with OSC 110 and OSC 111 when the program ends. A theme with no background leaves the terminal's own.
- **Preview and undo.** A cursor move in the picker applies the theme with no save. Esc puts back the styles from before the first preview. These styles stay in memory, so the undo reads no file. The picker session keeps a cache of the resolved themes.
- **Watch.** With `theme_watch` (on by default), a `tea.Tick` every 2 seconds reads the mod time of the active local theme file. That is one stat for each tick. A changed time reloads the file. A file that does not load shows its error, and the theme stays. Each loop has a sequence number, so a restart ends the old loop at its next tick.
- `Ctrl+T` opens the picker. The help overlay and the docs tell about it.

## Consequences
- A theme carries `Key`, the name that loads it again, and `Path`, its file. The display name of a bundled theme ("Tokyo Night") is not its key ("tokyo-night").
- Ctrl+T in the message input no longer swaps two characters (the transpose of the bubbles text area).
- A terminal that does not accept OSC 10 and OSC 11 shows its own colors, as before.
- The plan named a hint in the action bar on the first run. It is not done: the help overlay, `Ctrl+T` and the docs tell about the picker, and a hint needs a state file.

## Related
- [ADR-0015](/decisions/adr-0015-reusable-bubbles-in-internal-tui-ui--01a1f782e7a8.md)
