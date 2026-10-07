---
title: Keybindings
description: Keyboard shortcuts, mouse actions, and slash commands for Chit TUI.
---

Chit TUI is keyboard-first: every action has a keybinding, and the most
common ones are also clickable on the bottom action bar. Press `?` (from the
history or thread pane) for the built-in help overlay, which lists the same
keys as this page.

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
| `?` | Help overlay (only when the history or thread pane is focused) |
| `Esc` | Close overlay → leave thread → from the input, cancel an edit in progress and move focus to history |

`?` only opens help outside the input, since there it is a character you
might be typing.

## The Palette

`Ctrl+K` opens the unified palette. With an empty query it lists channels and
DMs sorted by mentions, then unread count, then most recent post — typing
fuzzy-filters the list. The first character switches modes:

| Prefix | Mode |
|--------|------|
| *(none)* | Jump to a channel or DM |
| `@` | Search people; `Enter` opens (or creates) a DM |
| `/` | Slash commands — the client's own and the server's |
| `?` | Full-text message search in the **active channel** |
| `??` | Full-text message search in **every channel** you belong to |

| Key | Action |
|-----|--------|
| (any text) | Filter / type query |
| `↑` / `↓` | Move selection |
| `Enter` | Select row (in `?`/`@` mode with no results: run the search) |
| `Esc` | Close palette |

Choosing a search result jumps to that message, opening its channel first if
it is somewhere else. A search query may include `#tag` to narrow results to
posts with that tag.

Choosing a slash command does not run it: it is put in the input box as
`/command `, because most commands take arguments and the palette has no way
to collect them. Finish the line and press `Enter`.

## Mouse Actions

| Mouse | Action |
|-------|--------|
| Wheel | Scroll the history/thread pane when the pointer is over it; move the palette cursor |
| Click a post in the history | Select it (same as moving there with `j`/`k`) |
| Drag in the history | Select lines; `y` copies them |
| Click a bar button | Trigger that action (same as its keybinding) |
| Click a palette row | Select it |
| Click outside the palette | Close it |
| Any click | Closes the help overlay |

Bar buttons ignore clicks while an overlay is open, so a click cannot stack a
second overlay on top of the first.

## History Pane (Viewport)

When the message history is focused:

| Key | Action |
|-----|--------|
| `j` / `↓` | Move post selection down |
| `k` / `↑` | Move post selection up |
| `Enter` | Open the thread for the selected post (fills the main pane) |
| `t` | Open tag picker for the selected post |
| `y` | Copy the mouse selection, or the selected post if nothing is selected |
| `e` | Edit the selected post (your own posts only) |
| `d` `d` | Delete the selected post (your own posts only); press `d` twice |
| `p` | Pin or unpin the selected post |

Reaching the top of the loaded history fetches the page before it, so older
messages load as you scroll back.

**Copying** uses the OSC 52 escape sequence, which the terminal itself acts
on — that is what makes it work over SSH and inside multiplexers. Not every
terminal supports it (Terminal.app ignores it; tmux and screen need it
enabled), so the status bar confirms how many lines were sent rather than
leaving you to guess.

**Editing** loads the post back into the input, and the action bar shows
`editing — enter saves, esc cancels`. `Enter` replaces the post; `Esc` cancels
and clears the input. Edit and delete only act on your own posts, matching
what the server enforces; on anyone else's they do nothing.

**Deleting** cannot be undone and `d` is one stray keystroke away, so the
first `d` asks for confirmation in the status bar. Press `d` again on the same
post to delete it; any other key cancels.

**Pinning** is a channel-level act, so unlike edit and delete it works on
anyone's post.

## Thread Pane

Opening a thread replaces the history pane and moves focus to the input,
which now composes replies to the thread root. Press `Tab` to focus the
thread pane itself for the scrolling and tagging keys.

| Key | Action |
|-----|--------|
| `j`/`k`, `↑`/`↓`, `PgUp`/`PgDn` | Scroll the thread |
| `t` | Open tag picker for the thread's root post |
| `Enter` (in input) | Send reply |
| `Esc` | Return to the channel view |

## Input

When the input area is focused:

| Key | Action |
|-----|--------|
| (any text) | Type message (`@` triggers mention autocomplete) |
| `Enter` | Send message (or thread reply, or save an edit) |
| `Alt+Enter` | Insert newline |
| `Esc` | Cancel an edit in progress; move focus to history |

A message starting with `/` is treated as a command when you press `Enter`.
A bare `/` opens the palette in command mode instead, so you can browse.

### Mention Autocomplete

Typing `@` offers channel members plus `@all`, `@channel`, and `@here`:

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `Enter` / `Tab` | Insert the selected mention |
| `Esc` | Close the popup |

## Slash Commands

These commands are handled by chit-tui itself and never reach the server:

| Command | Action |
|---------|--------|
| `/theme` (alias `/skin`) | Choose a theme (see [Themes](/chit/tui/guides/themes/)) |
| `/group` | Start a group conversation with three or more people |
| `/nick <display name>` | Change your display name |
| `/username <handle>` | Change your username (breaks existing @mentions) |
| `/threads` | Threads you follow, in this team and in DMs |
| `/leave` | Leave the current channel |
| `/logout` | Sign out and clear the stored session |

Anything else starting with `/` is sent to the server, which owns the command
registry and decides whether it is a command; text that is not one is posted
as an ordinary message.

`/nick` and `/username` are separate because renaming your handle breaks
every @mention already written, while a display name can change freely.

## Thread Inbox

`/threads` lists the threads you follow in the active team:

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Move selection |
| `Enter` | Open the thread |
| `u` | Unfollow the selected thread |
| `Esc` | Close the inbox |

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

The member picker opens when creating a **private** channel, to choose its
members, and for `/group`, to choose the people in the conversation:

| Key | Action |
|-----|--------|
| (any text) | Type a name to search for |
| `Enter` | Search for the typed name; with no new name typed, add the selected member(s), or the highlighted one |
| `↑` / `↓` | Navigate search results |
| `Tab` | Toggle multi-select on the highlighted user |
| `Backspace` | Remove last selected user (when search input is empty) |
| `Esc` | Close the picker: a private channel is still created, without extra members; a `/group` conversation is not created |

`Enter` searches whenever the text has changed, even with people already
selected, so you can run several searches and pick from each.

## Tag Picker

When the tag picker overlay is open (`t` from the history or thread pane):

| Key | Action |
|-----|--------|
| (any text) | Filter tags by name |
| `↑` / `↓` | Navigate tag list |
| `Enter` | Toggle tag on/off for the selected post |
| `Ctrl+N` | Create a new tag from the filter text |
| `Esc` | Close tag picker |

## Theme Picker

When the theme picker is open (`/theme`):

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Move selection |
| `Enter` | Apply and save the theme |
| `Esc` | Close without changing the theme |

## Help Overlay

`?` (from the history or thread pane) or the `[? Help]` bar button opens the
help overlay. It lists the keys, the client-side slash commands, and the mouse
actions. Any key or click closes it.
