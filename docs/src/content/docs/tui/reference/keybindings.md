---
title: Keybindings
description: Keyboard shortcuts, mouse actions, and slash commands for Chit TUI.
---

Chit TUI is keyboard-first. Every action has a key, and the most
common actions also have a button on the bottom action bar. Press `?` (from the
history or thread pane) for the built-in help overlay, which lists the same
keys as this page.

## Global Keybindings

These keys work in all panes:

| Key | Action |
|-----|--------|
| `Ctrl+C` | Quit |
| `Tab` / `Shift+Tab` | Toggle focus between the main pane and the input |
| `Ctrl+K` | Open the palette (jump to channels/DMs) |
| `Ctrl+S` | Open the palette in message-search mode (`?`) |
| `Ctrl+D` | Open the palette in people mode (`@`) |
| `Ctrl+N` | Create a new team channel |
| `Ctrl+T` | Open the theme picker |
| `?` | Help overlay (only when the history or thread pane is focused) |
| `Esc` | Close overlay → leave thread → from the input, cancel an edit in progress and move focus to history |

`?` opens help only outside the input. In the input, `?` is a character that
you can type.

## The Palette

`Ctrl+K` opens the unified palette. With an empty query it lists channels and
DMs sorted by mentions, then unread count, then most recent post. When you
type, the palette fuzzy-filters the list. The first character switches modes:

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

When you choose a search result, the palette goes to that message. If the
message is in a different channel, the palette first opens that channel. A search query may include `#tag` to narrow results to
posts with that tag.

When you choose a slash command, the palette does not run it. The palette
puts it in the input box as `/command `. Most commands take arguments, and the
palette cannot collect them. Finish the line and press `Enter`.

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

When the message history has focus:

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

At the top of the loaded history, the TUI gets the page before it, so older
messages load as you scroll back.

**Copy** uses the OSC 52 escape sequence, and the terminal itself acts
on it, so copy works over SSH and inside multiplexers. Not every
terminal supports OSC 52. Terminal.app ignores it, and tmux and screen must
have it enabled. For this reason, the status bar shows how many lines the TUI sent, so you
do not have to guess.

**Edit** loads the post back into the input, and the action bar shows
`editing — enter saves, esc cancels`. `Enter` replaces the post; `Esc` cancels
and clears the input. Edit and delete only act on the posts that you wrote, as
the server enforces. On the posts of other users they do nothing.

**Delete** cannot be undone, and one stray keystroke can press `d`. For this
reason, the first `d` shows a prompt in the status bar. Press `d` again on the same
post to delete it; any other key cancels.

**Pinning** is a channel-level act, so unlike edit and delete it works on
anyone's post.

## Thread Pane

When you open a thread, the thread replaces the history pane. Focus moves to
the input, which now writes replies to the thread root. To use the scroll and
tag keys, press `Tab` to move focus to the thread pane.

| Key | Action |
|-----|--------|
| `j`/`k`, `↑`/`↓`, `PgUp`/`PgDn` | Scroll the thread |
| `t` | Open tag picker for the thread's root post |
| `Enter` (in input) | Send reply |
| `Esc` | Return to the channel view |

## Input

When the input area has focus:

| Key | Action |
|-----|--------|
| (any text) | Type message (`@` triggers mention autocomplete) |
| `Enter` | Send message (or thread reply, or save an edit) |
| `Alt+Enter` | Insert newline |
| `Esc` | Cancel an edit in progress; move focus to history |

When you press `Enter`, the TUI treats a message that starts with `/` as a
command.
A bare `/` opens the palette in command mode instead, so you can browse.

### Mention Autocomplete

When you type `@`, the input offers channel members plus `@all`, `@channel`,
and `@here`:

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `Enter` / `Tab` | Insert the selected mention |
| `Esc` | Close the popup |

## Slash Commands

chit-tui itself runs these commands, and they never reach the server:

| Command | Action |
|---------|--------|
| `/theme` (alias `/skin`) | Open the theme picker (see [Themes](/chit/tui/guides/themes/)) |
| `/theme <name>` | Apply and save one theme |
| `/theme reload` | Read the active theme file again |
| `/group` | Start a group conversation with three or more people |
| `/nick <display name>` | Change your display name |
| `/username <handle>` | Change your username (breaks existing @mentions) |
| `/threads` | Threads you follow, in this team and in DMs |
| `/leave` | Leave the current channel |
| `/logout` | Sign out and clear the stored session |

chit-tui sends all other text that starts with `/` to the server. The server
owns the command registry and decides whether the text is a command. If the
text is not a command, the server posts it as an ordinary message.

`/nick` and `/username` are separate commands. A new handle breaks every
@mention that people wrote before, but a display name can change freely.

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

The member picker opens when you create a **private** channel, to choose its
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
| `Ctrl+N` | Create a new tag from the filter text, and apply it |
| `Ctrl+T` | Switch what `Ctrl+N` creates: a **channel** tag (the default), shared with the channel's members, or a **private** tag that only you see |
| `Esc` | Close tag picker |

The picker lists the channel's tags and the private tags of the user, which it marks
`(private)`. A `#tag` written in a message is always the channel's tag of
that name, created in the channel if it does not exist.

## Theme Picker

When the theme picker is open (`Ctrl+T` or `/theme`):

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Move the selection and show that theme |
| `Enter` | Keep the theme and save it |
| `Esc` | Close, and put back the theme from before the picker |

## Help Overlay

`?` (from the history or thread pane) or the `[? Help]` bar button opens the
help overlay. It lists the keys, the client-side slash commands, and the mouse
actions. Any key or click closes it.
