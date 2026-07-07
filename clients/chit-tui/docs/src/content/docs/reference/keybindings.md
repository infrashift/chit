---
title: Keybindings
description: Keyboard shortcuts for Chit TUI.
---

Chit TUI is fully keyboard-driven. All navigation and actions are performed through keybindings.

## Global Keybindings

These work regardless of which pane is focused:

| Key | Action |
|-----|--------|
| `Ctrl+C` | Quit |
| `Tab` | Focus next pane |
| `Shift+Tab` | Focus previous pane |
| `Ctrl+T` | Toggle thread panel |
| `Ctrl+K` | Open command palette |
| `Ctrl+S` | Open search |
| `Ctrl+N` | Create a new team channel |
| `Ctrl+D` | Create a new direct message or group channel |
| `Esc` | Close active overlay or thread |

## Focus Order

Pressing `Tab` cycles focus through panes in this order:

1. **Sidebar** — Team and channel lists
2. **Viewport** — Message list
3. **Input** — Message composition
4. **Thread** — Thread panel (only when visible)

`Shift+Tab` cycles in reverse. The command palette, search overlay, and thread panel are separate focus targets that don't participate in the Tab cycle.

## Sidebar

When the sidebar is focused:

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Select team or channel |
| `Esc` / `Backspace` | Go back to team list |

## Viewport

When the viewport is focused:

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `Enter` | Open thread for selected post |
| `t` | Open tag picker for selected post |

## Input

When the input area is focused:

| Key | Action |
|-----|--------|
| (any text) | Type message |
| `Enter` | Send message |
| `/` | Trigger command palette (when at start of line) |

## Thread Panel

When the thread panel is focused:

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll thread |
| `k` / `↑` | Scroll thread |
| `Enter` | Send reply |
| `Esc` | Close thread |

## Command Palette

When the command palette is open:

| Key | Action |
|-----|--------|
| (any text) | Filter commands |
| `↑` / `↓` | Navigate results |
| `Enter` | Select command |
| `Esc` | Close palette |

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

## DM Picker

When the DM picker overlay is open (`Ctrl+D`):

| Key | Action |
|-----|--------|
| (any text) | Search for users by name |
| `↑` / `↓` | Navigate search results |
| `Enter` | Select user (1:1 DM) or create group (if 2+ selected) |
| `Tab` | Toggle multi-select on highlighted user (for group channels) |
| `Backspace` | Remove last selected user (when search input is empty) |
| `Esc` | Close picker |

## Tag Picker

When the tag picker overlay is open (`t` from the viewport):

| Key | Action |
|-----|--------|
| (any text) | Filter tags by name |
| `↑` / `↓` | Navigate tag list |
| `Enter` | Toggle tag on/off for the selected post |
| `Ctrl+N` | Create a new tag from the filter text |
| `Esc` | Close tag picker |

## Search Overlay

When the search overlay is open:

| Key | Action |
|-----|--------|
| (any text) | Type search query (use `#tag` to filter by tags) |
| `Enter` | Submit search (or select result) |
| `↑` / `↓` | Navigate results |
| `Esc` | Close search |
