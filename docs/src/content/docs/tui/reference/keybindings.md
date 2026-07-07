---
title: Keybindings
description: Keyboard shortcuts and mouse actions for Chit TUI.
---

Chit TUI is keyboard-first: every action has a keybinding, and the most
common ones are also clickable on the bottom action bar. Press `?` (from the
history pane) for the built-in help overlay.

## Global Keybindings

These work regardless of which pane is focused:

| Key | Action |
|-----|--------|
| `Ctrl+C` | Quit |
| `Tab` / `Shift+Tab` | Toggle focus between the main pane and the input |
| `Ctrl+K` | Open the palette (jump to channels/DMs) |
| `Ctrl+S` | Open the palette in message-search mode (`?`) |
| `Ctrl+D` | Open the palette in people mode (`@`) |
| `Ctrl+N` | Create a new team channel |
| `?` | Help overlay (only when the history/thread pane is focused) |
| `Esc` | Close overlay → leave thread → move focus from input to history |

## The Palette

`Ctrl+K` opens the unified palette. With an empty query it lists channels and
DMs sorted by mentions, then unread count, then most recent post — typing
fuzzy-filters the list. The first character switches modes:

| Prefix | Mode |
|--------|------|
| *(none)* | Jump to a channel or DM |
| `@` | Search people; `Enter` opens (or creates) a DM |
| `/` | Slash commands |
| `?` | Full-text message search in the **active channel** |

| Key | Action |
|-----|--------|
| (any text) | Filter / type query |
| `↑` / `↓` | Move selection |
| `Enter` | Select row (in `?`/`@` mode with no results: run the search) |
| `Esc` | Close palette |

## Mouse Actions

| Mouse | Action |
|-------|--------|
| Wheel | Scroll the history/thread pane; move the palette cursor |
| Click a bar button | Trigger that action (same as its keybinding) |
| Click a palette row | Select it |
| Click outside the palette | Close it |

## History Pane (Viewport)

When the message history is focused:

| Key | Action |
|-----|--------|
| `j` / `↓` | Move post selection down |
| `k` / `↑` | Move post selection up |
| `Enter` | Open the thread for the selected post (fills the main pane) |
| `t` | Open tag picker for the selected post |

## Thread Pane

Opening a thread replaces the history pane; the input box now composes
replies to the thread root.

| Key | Action |
|-----|--------|
| `j`/`k`, `↑`/`↓`, `PgUp`/`PgDn` | Scroll the thread |
| `Enter` (in input) | Send reply |
| `Esc` | Return to the channel view |

## Input

When the input area is focused:

| Key | Action |
|-----|--------|
| (any text) | Type message (`@` triggers mention autocomplete) |
| `Enter` | Send message (or thread reply) |
| `Alt+Enter` | Insert newline |
| `/` | At the start of a message, `Enter` opens the palette in command mode |

## Channel Creator

When the channel creator overlay is open (`Ctrl+N`):

| Key | Action |
|-----|--------|
| (any text) | Type in the active field |
| `Tab` | Next field |
| `Shift+Tab` | Previous field |
| `←` / `→` | Toggle channel type (Open / Private) |
| `Enter` | Submit the new channel |
| `Esc` | Cancel and close |

## Member Picker

When creating a **private** channel, the member picker opens to select its
members:

| Key | Action |
|-----|--------|
| (any text) | Search for users by name |
| `↑` / `↓` | Navigate search results |
| `Tab` | Toggle multi-select on the highlighted user |
| `Enter` | Add the selected member(s) |
| `Backspace` | Remove last selected user (when search input is empty) |
| `Esc` | Skip adding members |

## Tag Picker

When the tag picker overlay is open (`t` from the history pane):

| Key | Action |
|-----|--------|
| (any text) | Filter tags by name |
| `↑` / `↓` | Navigate tag list |
| `Enter` | Toggle tag on/off for the selected post |
| `Ctrl+N` | Create a new tag from the filter text |
| `Esc` | Close tag picker |

## Help Overlay

`?` (from the history or thread pane) or the `[? Help]` bar button opens the
help overlay. Any key or click closes it.
