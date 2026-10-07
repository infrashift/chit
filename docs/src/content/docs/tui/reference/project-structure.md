---
title: Project Structure
description: Directory layout and file organization of Chit TUI.
---

## Top Level

```
clients/chit-tui/            (in the Chit monorepo)
├── cmd/chit-tui/             Entry point, flags, theme resolution
├── internal/                 All application code
├── tests/e2e/                End-to-end tests against a real server
├── Containerfiles/           Image for the e2e test run
├── scripts/                  e2e container entrypoint
├── docs/history/             Completed TODO lists and the phase PRDs/specs
├── Makefile                  Build, test, lint, cover, e2e targets
├── go.mod                    Go module definition (nested module)
├── .golangci.yml             Linter configuration
├── README.md
├── CHANGELOG.md
└── TODO.md                   Open work only
```

### `cmd/chit-tui/`

| File | Purpose |
|------|---------|
| `main.go` | Loads config, resolves the theme, restores the session, starts the program |
| `flags.go` | `--theme`, `--appearance`, `--list-themes`; builds the theme request |

The theme is resolved here, before the Bubble Tea program starts: asking the
terminal whether it is dark sends an escape sequence, and the reply would
otherwise arrive as typed input.

## `internal/` Packages

### `config/`

Configuration loading and validation.

| File | Purpose |
|------|---------|
| `config.go` | `Config` struct, `Load()` (environment > file > default), `Validate()`, `WSURL()`, `ThemeSettingKey()` |
| `toml.go` | `FilePath()` (honors `CHIT_CONFIG_FILE`), `ThemesDir()`, `LoadFile()` |
| `cue.go` | Validates the config file against the schema, turning bad keys into warnings |
| `theme.go` | `ValidateThemeTOML()` — strict validation of user theme files |
| `save.go` | `SaveThemeSetting()` — rewrites one theme line, keeping comments |
| `schema/config.cue` | Schema for `config.toml` |
| `schema/theme.cue` | Schema for `themes/<name>.toml` |

The schemas are embedded in the binary.

### `model/`

Data transfer objects mirroring the Chit server's JSON API. No server code is imported.

| File | Types |
|------|-------|
| `user.go` | `User` |
| `team.go` | `Team` |
| `channel.go` | `Channel`, `ChannelMember` |
| `post.go` | `Post`, `PostList` |
| `thread.go` | `Thread`, `ThreadResponse`, `UserThreadList` |
| `command.go` | `Command` |
| `tag.go` | `Tag` |
| `websocket.go` | `WebSocketEvent`, `WebSocketBroadcast`, `WebSocketMessage`, `AppError` |
| `helpers.go` | `MillisToTime()` |

### `api/`

HTTP client for the Chit REST API.

| File | Purpose |
|------|---------|
| `client.go` | `ChitClient` interface, `httpClient` struct, all endpoint methods, 30s `DefaultTimeout` |
| `transport.go` | `authTransport` — injects the token under the configured header |
| `errors.go` | `APIError`, `IsUnauthorized`, `parseErrorResponse` |

### `auth/`

Sign-in and session storage.

| File | Purpose |
|------|---------|
| `kratos.go` | `KratosClient` — login flow and session checks against `/kratos` |
| `persist.go` | `SessionStore` — the session file, written atomically with mode `0600` |
| `tokenstore.go` | `TokenStore` — the current token, read by the REST client on every request |

### `ws/`

WebSocket client: one session per `Connect`, keepalive pings, and redialing.

| File | Purpose |
|------|---------|
| `client.go` | `WSClient` interface, `ConnState`, `wsClient` — session loop, ping, redial |
| `reconnect.go` | `Backoff` (capped, jittered exponential backoff), `ErrUnauthorized` |

### `testutil/`

Shared test helpers.

| File | Purpose |
|------|---------|
| `testutil.go` | `StripANSI` for asserting on rendered output, `Styles` for a default style set |

### `tui/`

The Bubble Tea TUI layer. The root `Model` is one type whose methods are
split by concern across files; new handlers go beside the ones they resemble
rather than growing `update.go`.

| File | Purpose |
|------|---------|
| `model.go` | `Model` struct, `NewModel`, `Init`, focus areas, status messages |
| `update.go` | `Update` — the top-level message dispatcher |
| `keys.go` | Key routing: overlays first, then global bindings, then the focused pane |
| `mouse.go` | Mouse routing — wheel scroll, drag selection, action-bar and palette clicks |
| `slash.go` | Client-side slash commands (`/theme`, `/group`, `/nick`, …) |
| `view.go` | `View`, the overlay table, `placeOverlay` compositing |
| `focus.go` | Focus changes, focus cycling, delegating keys to the focused pane |
| `channels.go` | Channel and DM bookkeeping |
| `unread.go` | Unread and mention counts |
| `history.go` | Channel history, threads, editing, loading older pages |
| `users.go` | User lookups, mention autocomplete entries |
| `tags.go` | Tag lookups and tagging posts |
| `wsevents.go` | WebSocket events and connection-state changes |
| `session.go` | Sign-in, sign-out, session expiry |
| `clipboard.go` | Copy via OSC 52 |
| `commands.go` | `tea.Cmd` wrappers (`FetchMe`, `FetchPosts`, `ListenWebSocket`, etc.) |
| `messages.go` | All `tea.Msg` types (`UserLoadedMsg`, `PostsLoadedMsg`, etc.) |
| `keymap.go` | `KeyMap` struct with global keybindings |
| `doc.go` | Package documentation |

### `tui/palette/`

The unified palette overlay (Ctrl+K): jump to channels/DMs sorted by unread
activity, `@` people search, `/` slash commands, `?` message search in the
active channel, `??` everywhere. Includes a hand-rolled fuzzy subsequence
matcher (`fuzzy.go`).

### `tui/actionbar/`

Bottom action/status bar: clickable buttons (hit-tested by cell span),
active team/channel context, the editing indicator, transient errors, and the
WebSocket connection indicator.

### `tui/help/`

Static help overlay listing keybindings, client-side slash commands, and mouse
actions.

### `tui/login/`

The email/password login screen shown before a session exists and after one
expires.

### `tui/viewport/`

Post list viewport wrapping `bubbles/viewport`. Renders each post with the
`post` component, vertically, and owns the post cursor, mouse line selection, and search-match
highlighting.

### `tui/input/`

Message input wrapping `bubbles/textarea`. Detects `/` prefix for slash command triggers.

### `tui/thread/`

Read-only thread view that swaps into the main pane; replies are composed in
the regular input box.

### `tui/threadinbox/`

Overlay listing the threads you follow in the active team (`/threads`), with
unfollow.

### `tui/post/`

Single post renderer using Glamour markdown. Shows username, timestamp, pinned badge, edited badge, encrypted badge, reply count badge, and tag badges.

### `tui/mention/`

@mention autocomplete popup. Parses `@` trigger in input, shows channel members and special mentions (`@all`, `@channel`, `@here`), and highlights mentions in posts.

### `tui/dmpicker/`

Member-selection overlay used when creating private channels and for `/group`
(multi-select with Tab). Starting DMs is handled by the palette's `@` mode.

### `tui/skinpicker/`

Runtime theme switcher overlay (`/theme`). Lists bundled themes, your TOML
themes, and legacy JSON skins for hot-swapping without restart.

### `tui/chcreator/`

Channel creator overlay with display name, auto-slug, type toggle (Open/Private), and purpose fields.

### `tui/tagpicker/`

Tag picker overlay for managing post tags. Filter by prefix, toggle tags on/off, create new tags. Also provides `ExtractHashtags` and `StripHashtags` for `#tag` input parsing.

### `tui/ui/listwin/`

Decides which rows of a list an overlay draws, so the cursor stays on screen
however long the list is. Shared by the pickers.

### `tui/ui/markdown/`

Theme-aware Glamour style config builder. Maps theme colors to markdown heading, link, code, and syntax highlighting styles.

### `tui/ui/theme/`

Theme definitions, loading, and resolution.

| File | Purpose |
|------|---------|
| `theme.go` | `Theme` struct, derived colors, `ListAvailable`, legacy JSON loading (`ParseJSON`, `LoadFromFile`) |
| `builtin.go` | The eight bundled themes, `Lookup`, `BuiltinNames` |
| `local.go` | `LoadLocal` for TOML themes, terminal color names, `ResolveNamed` |
| `resolve.go` | `Resolve` — flag, config, and appearance precedence |
| `color.go` | Hex parsing and color blending |

### `tui/ui/styles/`

Lipgloss style constructors that accept a `Theme` and produce a `Styles` struct used by all components.

## Tests

Unit tests sit beside the code they test. Functional flows that should run
through a real Bubble Tea program are in `internal/tui/flow_test.go`, using
[teatest](https://github.com/charmbracelet/x/tree/main/exp/teatest). Tests
never touch your real home directory: packages that read or write config,
sessions, or themes point `HOME`, `XDG_CONFIG_HOME`, and `CHIT_CONFIG_FILE` at
a temporary directory first.

`tests/e2e/` holds end-to-end tests of the API and WebSocket clients against a
real Chit server. They carry the `e2e` build tag, so `make test` skips them;
`make test-e2e` runs them in a podman pod with Postgres and the server.

## Makefile Targets

| Target | Command |
|--------|---------|
| `make all` | lint + test + build |
| `make build` | `go build -o bin/chit-tui ./cmd/chit-tui` |
| `make test` | `go test -race ./...` |
| `make cover` | Test with coverage report |
| `make lint` | `golangci-lint run ./...` |
| `make clean` | Remove `bin/` and the coverage file |
| `make test-e2e` | Build images and run the e2e suite in a podman pod |
| `make test-e2e-clean` | Remove a leftover e2e pod |

The Go targets run with `GOWORK=off`, so the module builds from its own
`go.mod` even though it sits inside the Chit repository's Go workspace.
