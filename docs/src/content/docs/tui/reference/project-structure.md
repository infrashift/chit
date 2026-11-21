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
| `input_filter.go` | `filterInput` — drops terminal reply sequences from key input, and marks a key of several runes as pasted |
| `doc.go` | Package documentation |

The command resolves the theme here, before the Bubble Tea program starts. To
ask the terminal whether it is dark, the command sends an escape sequence. If
the program runs at that time, the reply arrives as typed input.

## `internal/` Packages

### `config/`

Loads and validates the configuration.

| File | Purpose |
|------|---------|
| `config.go` | `Config` struct, `Load()` (environment > file > default), `Validate()`, `WSURL()`, `ThemeSettingKey()` |
| `toml.go` | `FilePath()` (honors `CHIT_CONFIG_FILE`), `ThemesDir()`, `LoadFile()` |
| `cue.go` | Validates the config file against the schema, turning bad keys into warnings |
| `theme.go` | `ValidateThemeTOML()` — strict validation of user theme files |
| `save.go` | `SaveThemeSetting()` — rewrites one theme line, keeping comments |
| `schema/config.cue` | Schema for `config.toml` |
| `schema/theme.cue` | Schema for `themes/<name>.toml` |

The binary embeds the schemas.

### `model/`

Data transfer objects that mirror the Chit server's JSON API. The package does not import server code.

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
| `client.go` | `ChitClient` interface, `httpClient` struct, `NewClient`, `NewClientWithHeader`, `NewClientWithTokenSource`, all endpoint methods, 30s `DefaultTimeout` |
| `transport.go` | `TokenSource` interface, `authTransport` — injects the token under the configured header |
| `errors.go` | `APIError`, `IsUnauthorized`, `parseErrorResponse` |

### `auth/`

Sign-in and session storage.

| File | Purpose |
|------|---------|
| `kratos.go` | `KratosClient` — login flow and session checks against `/kratos` |
| `persist.go` | `SessionStore` — the session file, written atomically with mode `0600` |
| `tokenstore.go` | `TokenStore` — the current token, read by the REST client on every request |

### `ws/`

WebSocket client: one session per `Connect`, keepalive pings, and redial.

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

The Bubble Tea TUI layer. The root `Model` is one type. Its methods are in
different files, one file for each concern. Put each new handler beside the
handlers that it resembles, and do not make `update.go` larger.

| File | Purpose |
|------|---------|
| `model.go` | `Model` struct, `NewModel`, `Init`, focus areas, status messages |
| `update.go` | `Update` — the top-level message dispatcher, and the routers (`routeSessionMsg`, `routeChannelMsg`, `routePostMsg`, `routeTagMsg`, `routeLookupMsg`) that send each message to its handler |
| `keys.go` | Key routing: overlays first, then global bindings, then the focused pane |
| `mouse.go` | Mouse routing — wheel scroll, drag selection, action-bar and palette clicks |
| `slash.go` | Client-side slash commands (`/theme`, `/group`, `/nick`, …) |
| `view.go` | `View`, the overlay table, `placeOverlay` compositing |
| `focus.go` | Focus changes, focus cycling, delegating keys to the focused pane |
| `channels.go` | Channel and DM bookkeeping |
| `unread.go` | Unread and mention counts |
| `history.go` | Channel history, send, edit, search hits, loading older pages, leaving a thread |
| `threads.go` | Thread loading, the thread inbox (`/threads`), opening the thread of the selected post |
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

The unified palette overlay (Ctrl+K). It goes to channels and DMs, sorted by
unread activity. It also has `@` people search, `/` slash commands, `?`
message search in the active channel, and `??` search everywhere. It includes
a hand-rolled fuzzy matcher of letters in sequence (`fuzzy.go`).

### `tui/actionbar/`

Bottom action/status bar: clickable buttons (hit-tested by cell span),
active team/channel context, the edit indicator, transient errors, and the
WebSocket connection indicator.

### `tui/help/`

Static help overlay listing keybindings, client-side slash commands, and mouse
actions.

### `tui/login/`

The email/password login screen shown before a session exists and after one
expires.

### `tui/viewport/`

Post list viewport around `bubbles/viewport`. It renders each post with the
`post` component, vertically. It owns the post cursor, the mouse line
selection, and the highlights of search matches.

### `tui/input/`

Message input around `bubbles/textarea`. Detects `/` prefix for slash command triggers.

### `tui/thread/`

Read-only thread view that swaps into the main pane. The user writes replies
in the regular input box.

### `tui/threadinbox/`

Overlay listing the threads you follow in the active team (`/threads`), with
unfollow.

### `tui/post/`

Single post renderer with Glamour Markdown. Shows username, timestamp, pinned badge, edited badge, encrypted badge, reply count badge, and tag badges.

### `tui/mention/`

@mention autocomplete popup. Parses `@` trigger in input, shows channel members and special mentions (`@all`, `@channel`, `@here`), and highlights mentions in posts.

### `tui/dmpicker/`

Member-selection overlay for new private channels and for `/group`
(multi-select with Tab). The `@` mode of the palette starts DMs.

### `tui/skinpicker/`

Runtime theme switcher overlay (`/theme`). Lists bundled themes, local TOML
themes, and legacy JSON skins. The new theme applies without restart.

### `tui/chcreator/`

Channel creator overlay with display name, auto-slug, type toggle (Open/Private), and purpose fields.

### `tui/tagpicker/`

Tag picker overlay for the tags of a post. Filter by prefix, toggle tags on/off, create new tags. Also provides `ExtractHashtags` and `StripHashtags`, which parse `#tag` input.

### `tui/ui/listwin/`

Decides which rows of a list an overlay draws, so the cursor stays on screen
however long the list is. Shared by the pickers.

### Reusable Bubbles in `tui/ui/`

Each of these packages is a Bubble that more than one feature uses. A Bubble
imports no package of the TUI other than `assert` and `ui/styles`. It has no
fields of function type. It sends its results as `tea.Msg` types, and the
root model routes them in `bubbles.go` (ADR-0015).

| Package | Purpose | Users |
|---------|---------|-------|
| `ui/confirm` | One y/N question in the action bar. The yes key answers Yes, and each other key closes the question. The question also closes after 10 seconds. | Post delete |
| `ui/chips` | An ordered list of chips with a cap and no duplicates. Backspace removes the last chip. The view wraps to the width. | The member picker |
| `ui/prompt` | A one-line text overlay. Enter sends the value, and the parent checks it or shows an error. Esc cancels. | Tag rename, the alt text of an attachment |
| `ui/activity` | A capped set of short-lived items on one line, each with a timer | The `stream_status` indicator |
| `ui/infobox` | An overlay of label and value rows that scrolls, with action keys that send `ActionMsg` | The post details (`i`) |
| `ui/filepick` | The file picker of bubbles in an overlay, with a list of file types and a size cap | `/attach` with no path |

### `tui/ui/markdown/`

Theme-aware Glamour style config builder. Maps theme colors to the Markdown styles of headings, links, code, and syntax highlights.

### `tui/ui/theme/`

Theme definitions, the theme loader, and resolution.

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

Unit tests sit beside the code that they test. Functional flows that run
through a real Bubble Tea program are in `internal/tui/flow_test.go`. They use
[teatest](https://github.com/charmbracelet/x/tree/main/exp/teatest). Tests
never touch the real home directory. Packages that read or write config,
sessions, or themes first point `HOME`, `XDG_CONFIG_HOME`, and
`CHIT_CONFIG_FILE` at a temporary directory.

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
