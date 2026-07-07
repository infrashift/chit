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

- **Left sidebar** — Your teams and channels
- **Center viewport** — Messages in the active channel
- **Bottom input** — Message composition area

## 4. Navigate

| Action | Key |
|--------|-----|
| Move between panes | `Tab` / `Shift+Tab` |
| Select team or channel | `Enter` |
| Go back to teams | `Esc` or `Backspace` |
| Navigate lists | `j`/`k` or arrow keys |

## 5. Send a Message

1. Press `Tab` until the input area is focused
2. Type your message (supports Markdown)
3. Press `Enter` to send

## 6. Open a Thread

1. Focus the viewport (`Tab` to it)
2. Navigate to a post with `j`/`k`
3. Press `Enter` to open the thread panel
4. Type a reply and press `Enter`

## 7. Search

Press `Ctrl+S` to open the search overlay. Type a query, press `Enter` to search, then use arrow keys to browse results.

## 8. Quit

Press `Ctrl+C` to exit.
