---
title: Project Structure
description: Directory layout and file organization of Chit TUI.
---

## Top Level

```
chit-tui/
├── cmd/chit-tui/main.go     Entry point
├── internal/                 All application code
├── docs/                     Documentation site (Astro + Starlight)
├── Makefile                  Build, test, lint, cover targets
├── go.mod                    Go module definition
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
| `app.go` | Root `Model` — composes all sub-components, message routing, focus management |
| `messages.go` | All `tea.Msg` types (`UserLoadedMsg`, `PostsLoadedMsg`, etc.) |
| `commands.go` | `tea.Cmd` wrappers (`FetchMe`, `FetchPosts`, `ListenWebSocket`, etc.) |
| `keymap.go` | `KeyMap` struct with global keybindings |

### `tui/sidebar/`

Team and channel navigation. Cursor-based list with unread badges.

### `tui/viewport/`

Post list viewport wrapping `bubbles/viewport`. Renders `PostBubble` components vertically.

### `tui/input/`

Message input wrapping `bubbles/textarea`. Detects `/` prefix for slash command triggers.

### `tui/thread/`

Thread side panel with its own viewport and reply input.

### `tui/post/`

Single post renderer using Glamour markdown. Shows username, timestamp, pinned badge, encrypted badge, reply count badge, and tag badges.

### `tui/cmdpalette/`

Command palette overlay with fuzzy filtering.

### `tui/search/`

Post search overlay with query input and result navigation. Supports `#tag` syntax to filter by tags.

### `tui/mention/`

@mention autocomplete popup. Parses `@` trigger in input, shows channel members and special mentions (`@all`, `@channel`, `@here`), and highlights mentions in posts.

### `tui/dmpicker/`

DM picker overlay for user search. Supports single-select (1:1 DM) and multi-select (group channel) modes.

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
