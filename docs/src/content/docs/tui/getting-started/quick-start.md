---
title: Quick Start
description: Get Chit TUI running in under a minute.
---

## 1. Set Server URL

Chit TUI needs to know where the Chit server (or Oathkeeper proxy) is:

```bash
export CHIT_SERVER_URL=http://localhost:4455
```

## 2. Launch

```bash
chit-tui
```

## 3. Log In

The TUI opens with a login screen. Enter your **email** and **password**, then
press `Enter`. After login, the session is stored locally and reused on
subsequent launches.

Once authenticated, you'll see:

- **Top pane** — Chat history for the active channel
- **Bottom input** — Message composition area
- **Action bar** — A row of clickable buttons, the active team/channel, and
  the connection indicator along the bottom edge

## 4. Navigate

All navigation goes through the **palette**:

| Action | Key |
|--------|-----|
| Open the palette (channels/DMs, sorted by unread activity) | `Ctrl+K` |
| Find a person / start a DM | `Ctrl+D` (or type `@` in the palette) |
| Run a slash command | type `/` in the palette |
| Search messages in the active channel | `Ctrl+S` (or type `?`) |
| Toggle focus between history and input | `Tab` |
| Show all keybindings | `?` (from the history pane) |

In the palette, type to filter, use `↑`/`↓` to move, and `Enter` to select.
The mouse works too: click the action-bar buttons or a palette row, and
wheel-scroll the history.

## 5. Send a Message

1. Type your message in the input (supports Markdown) — the input is focused
   by default; press `Tab` if the history pane has focus
2. Press `Enter` to send (`Alt+Enter` inserts a newline)

## 6. Open a Thread

1. Focus the history pane (`Tab`)
2. Move the selection to a post with `j`/`k`
3. Press `Enter` — the thread fills the main pane
4. Type a reply in the input and press `Enter`
5. Press `Esc` to return to the channel

## 7. Search

Press `Ctrl+S` — the palette opens in message-search mode. Type a query,
press `Enter` to search the active channel, then use arrow keys to browse
results.

## 8. Quit

Press `Ctrl+C` to exit.
