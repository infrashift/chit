---
title: Project Structure
description: Directory layout and file organization of Chit TUI.
---

## Top Level

```
clients/chit-tui/            (in the Chit monorepo)
├── cmd/chit-tui/main.go     Entry point
├── internal/                 All application code
├── Makefile                  Build, test, lint, cover targets
├── go.mod                    Go module definition (nested module)
├── .golangci.yml             Linter configuration
├── .gitignore
├── README.md
├── CHANGELOG.md
└── TODO.md                   Development task tracking
```

## `internal/` Packages

### `config/`

Environment variable loading and validation.

| File | Purpose |
|------|---------|
| `config.go` | `Config` struct, `Load()`, `Validate()`, `WSURL()` |
| `config_test.go` | Env var loading, validation, defaults |

### `model/`

Data transfer objects mirroring the Chit server's JSON API. No server code is imported.

| File | Types |
|------|-------|
| `user.go` | `User` |
| `team.go` | `Team`, `TeamMember` |
| `channel.go` | `Channel`, `ChannelMember` |
| `post.go` | `Post`, `PostList` |
| `thread.go` | `Thread`, `ThreadMembership`, `ThreadResponse` |
| `command.go` | `Command` |
| `tag.go` | `Tag` |
| `websocket.go` | `WebSocketEvent`, `WebSocketBroadcast`, `WebSocketMessage`, `AppError` |
| `helpers.go` | `MillisToTime()` |

### `api/`

HTTP client for the Chit REST API.

| File | Purpose |
|------|---------|
| `client.go` | `ChitClient` interface, `httpClient` struct, all endpoint methods |
| `transport.go` | `authTransport` — injects `X-Session-Token` header |
| `errors.go` | `APIError`, `parseErrorResponse` |

### `ws/`

WebSocket client with automatic reconnection.

| File | Purpose |
|------|---------|
| `client.go` | `WSClient` interface, `wsClient` impl, `readLoop`, `reconnect` |
| `reconnect.go` | `Backoff` struct (exponential backoff with cap) |

### `testutil/`

Shared test helpers.

| File | Purpose |
|------|---------|
| `testutil.go` | Factory functions (`NewTestUser`, etc.), `MockHandler`, `NewMockServerMux` |

### `tui/`

The Bubble Tea TUI layer.

| File | Purpose |
|------|---------|
| `app.go` | Root `Model` — composes all sub-components, message routing, focus and main-pane management, overlay compositing |
| `mouse.go` | Mouse routing — wheel scroll, action-bar hit-testing, palette clicks |
| `messages.go` | All `tea.Msg` types (`UserLoadedMsg`, `PostsLoadedMsg`, etc.) |
| `commands.go` | `tea.Cmd` wrappers (`FetchMe`, `FetchPosts`, `ListenWebSocket`, etc.) |
| `keymap.go` | `KeyMap` struct with global keybindings |

### `tui/palette/`

The unified palette overlay (Ctrl+K): jump to channels/DMs sorted by unread
activity, `@` people search, `/` slash commands, `?` message search. Includes
a hand-rolled fuzzy subsequence matcher (`fuzzy.go`).

### `tui/actionbar/`

Bottom action/status bar: clickable buttons (hit-tested by cell span),
active team/channel context, transient errors, and the WebSocket connection
indicator.

### `tui/help/`

Static help overlay listing keybindings and mouse actions.

### `tui/viewport/`

Post list viewport wrapping `bubbles/viewport`. Renders `PostBubble` components vertically.

### `tui/input/`

Message input wrapping `bubbles/textarea`. Detects `/` prefix for slash command triggers.

### `tui/thread/`

Read-only thread view that swaps into the main pane; replies are composed in
the regular input box.

### `tui/post/`

Single post renderer using Glamour markdown. Shows username, timestamp, pinned badge, encrypted badge, reply count badge, and tag badges.

### `tui/mention/`

@mention autocomplete popup. Parses `@` trigger in input, shows channel members and special mentions (`@all`, `@channel`, `@here`), and highlights mentions in posts.

### `tui/dmpicker/`

Member-selection overlay used when creating private channels (multi-select
with Tab). Starting DMs is handled by the palette's `@` mode.

### `tui/skinpicker/`

Runtime theme switcher overlay. Lists built-in and custom themes for hot-swapping without restart.

### `tui/chcreator/`

Channel creator overlay with display name, auto-slug, type toggle (Open/Private), and purpose fields.

### `tui/tagpicker/`

Tag picker overlay for managing post tags. Filter by prefix, toggle tags on/off, create new tags. Also provides `ExtractHashtags` and `StripHashtags` for `#tag` input parsing.

### `tui/ui/markdown/`

Theme-aware Glamour style config builder. Maps theme colors to markdown heading, link, code, and syntax highlighting styles.

### `tui/ui/theme/`

Theme definitions and loading.

| File | Purpose |
|------|---------|
| `theme.go` | `Theme` struct, built-in themes (`TokyoNight`, `Catppuccin`, `Kanagawa`, `Nightfox`), `ParseJSON`, `LoadFromFile`, `LoadNamed` |

### `tui/ui/styles/`

Lipgloss style constructors that accept a `Theme` and produce a `Styles` struct used by all components.

## Makefile Targets

| Target | Command |
|--------|---------|
| `make all` | lint + test + build |
| `make build` | `go build -o bin/chit-tui` |
| `make test` | `go test ./...` |
| `make cover` | Test with coverage report |
| `make lint` | `golangci-lint run` |
| `make clean` | Remove `bin/` |
