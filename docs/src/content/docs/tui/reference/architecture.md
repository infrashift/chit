---
title: Architecture
description: Technical architecture of Chit TUI.
---

Chit TUI follows the **Elm Architecture** (Model-Update-View) using the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

## Elm Architecture

Every component in Chit TUI implements three functions:

- **Model** — A struct holding all component state
- **Update** — A pure function: `(Model, Msg) → (Model, Cmd)`
- **View** — A pure function: `Model → string`

The root model composes all sub-components and routes messages to the appropriate child.

## Component Tree

```
Root Model (app.go)
├── Viewport        — Post list with scroll (main pane, channel view)
├── Thread          — Thread view (swaps into the main pane)
├── Input           — Message textarea (channel posts and thread replies)
├── Action Bar      — Bottom bar: clickable buttons, context, connection dot
├── Palette         — Unified overlay: channels/people/commands/search
├── Help            — Keybinding and mouse reference overlay
├── Mention         — @mention autocomplete popup
├── Member Picker   — Private-channel member selection (dmpicker)
├── Skin Picker     — Runtime theme switcher
├── Channel Creator — New channel form
└── Tag Picker      — Post tag management overlay
```

## Message Flow

1. **User presses a key** → Bubble Tea delivers a `tea.KeyMsg`
2. **Root Update** routes to a visible overlay first (help, palette, pickers)
3. Otherwise checks global keybindings (Tab, Ctrl+K, Ctrl+S, …)
4. If not a global key, **delegates** to the focused sub-component
5. Sub-component returns a **tea.Cmd** (async operation) if needed
6. Cmd executes and produces a **result Msg** (e.g. `PostsLoadedMsg`)
7. Root Update handles the result, updating state and triggering new Cmds

Mouse events (`tea.MouseMsg`) are routed by `handleMouse` (`mouse.go`): the
wheel scrolls the main pane or moves the palette cursor, and left-clicks are
hit-tested against the action bar's button spans or the palette's rows.
Every clickable action dispatches through the same code path as its
keybinding.

## The Unified Palette

The palette (`internal/tui/palette`) is the single navigation entry point,
opened with `Ctrl+K`. The first character of the query selects the mode:

| Prefix | Mode | Data source |
|--------|------|-------------|
| *(none)* | Jump to channel/DM | All-team channel list + DMs, fuzzy filtered |
| `@` | Find people | `SearchUsers` (debounced), Enter opens a DM |
| `/` | Slash commands | Server command list, fuzzy filtered |
| `?` | Message search | `SearchPosts`, scoped to the active channel |

With an empty query, channels sort by **mentions → unread → most recent
post**, so the channels needing attention are always on top. Unread `(N)` and
mention `[@N]` badges render per row. The palette shares the root model's
unread/mention maps by reference, so badge state needs no extra plumbing.

## API Integration

### HTTP Client

The `ChitClient` interface abstracts all REST API calls:

```go
type ChitClient interface {
    GetMe(ctx context.Context) (*model.User, error)
    GetMyTeams(ctx context.Context) ([]*model.Team, error)
    GetMyChannels(ctx context.Context, teamID string) ([]*model.Channel, error)
    GetChannelPosts(ctx context.Context, channelID string, page, perPage int) (*model.PostList, error)
    CreatePost(ctx context.Context, post *model.Post) (*model.Post, error)
    // ... and more
}
```

Authentication is handled by `authTransport`, an `http.RoundTripper` that injects the `X-Session-Token` header into every request.

On startup the TUI fetches channels for **every team** the user belongs to
(`channelsByTeam` on the root model); the palette shows a `· team` suffix on
channel rows when more than one team is loaded.

### WebSocket Client

The `WSClient` interface provides real-time event streaming:

```go
type WSClient interface {
    Connect() error
    Close() error
    Events() <-chan model.WebSocketEvent
    Send(msg model.WebSocketMessage) error
}
```

The WebSocket client runs a background `readLoop` goroutine. On connection failure, it automatically reconnects using exponential backoff (500ms base, 30s max). Events are delivered through a buffered channel that the TUI polls via `ListenWebSocket` tea.Cmd. The action bar shows a connection indicator (`●` connected / `○` disconnected).

### Stale Event Protection

The root model tracks a `lastWSSeq` counter. WebSocket events with a sequence number less than or equal to the last seen sequence are dropped, preventing duplicate or out-of-order message processing.

## Focus and Panes

The root model owns a `FocusArea` enum. Only the focused component receives
key messages. `Tab`/`Shift+Tab` toggle between the top pane and the input.

The top pane is controlled by a `mainPane` enum:

- **`paneChannel`** — the channel's post history (Viewport)
- **`paneThread`** — a thread opened with Enter on a post; the input then
  composes replies to the thread root, and `Esc` returns to the channel

Overlays (palette, help, pickers) intercept keys while visible and are
composited over the layout in z-order through a single overlay table using a
character-level, ANSI-aware `placeOverlay` function.

## Layout

The view is composed with Lipgloss:

```
┌───────────────────────────────────────────────┐
│                                               │
│        Viewport  /  Thread (main pane)        │
│                                               │
├───────────────────────────────────────────────┤
│                    Input                      │
├───────────────────────────────────────────────┤
│ [^K Jump] [^S Search] [^D DM] [^N New] [? Help] │ team > #channel   ● user │
└───────────────────────────────────────────────┘
```

Unread/mention state, DM display names, and the all-team channel lists are
owned by the root model; the palette and action bar render from that state.

## Own Model DTOs

Chit TUI defines its own data transfer objects in the `model` package, mirroring the server's JSON shapes. This avoids importing the server codebase (and its dependencies like `pgx`, `chi`, etc.) into the TUI.
