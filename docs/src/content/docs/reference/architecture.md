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
├── Sidebar        — Team/channel navigation
├── Viewport       — Post list with scroll
├── Input          — Message textarea
├── Thread         — Thread side panel
├── Command Palette — Fuzzy command overlay
├── Search         — Post search overlay
├── Mention        — @mention autocomplete popup
├── DM Picker      — Direct message user search
├── Skin Picker    — Runtime theme switcher
├── Channel Creator — New channel form
└── Tag Picker     — Post tag management overlay
```

## Message Flow

1. **User presses a key** → Bubble Tea delivers a `tea.KeyMsg`
2. **Root Update** checks global keybindings (Tab, Ctrl+T, Ctrl+K, etc.)
3. If not a global key, **delegates** to the focused sub-component
4. Sub-component returns a **tea.Cmd** (async operation) if needed
5. Cmd executes and produces a **result Msg** (e.g. `PostsLoadedMsg`)
6. Root Update handles the result, updating state and triggering new Cmds

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

The WebSocket client runs a background `readLoop` goroutine. On connection failure, it automatically reconnects using exponential backoff (500ms base, 30s max). Events are delivered through a buffered channel that the TUI polls via `ListenWebSocket` tea.Cmd.

### Stale Event Protection

The root model tracks a `lastWSSeq` counter. WebSocket events with a sequence number less than or equal to the last seen sequence are dropped, preventing duplicate or out-of-order message processing.

## Focus Management

The root model owns a `FocusArea` enum. Only the focused component receives key messages. Focus areas:

- `Sidebar` — Team/channel navigation
- `Viewport` — Post list
- `Input` — Message composition
- `Thread` — Thread panel (when visible)
- `CmdPalette` — Command palette overlay
- `Search` — Search overlay
- `DMPicker` — DM user search overlay
- `SkinPicker` — Theme switcher overlay
- `ChCreator` — Channel creation form overlay
- `TagPicker` — Post tag management overlay

Tab/Shift+Tab cycles through `Sidebar → Viewport → Input → Thread` (when visible). Overlays (palette, search, tag picker, etc.) are activated by their respective keybindings.

## Layout

The view is composed with Lipgloss:

```
┌──────────┬────────────────────────┬──────────┐
│          │                        │          │
│ Sidebar  │      Viewport          │  Thread  │
│          │                        │ (toggle) │
│          │                        │          │
│          ├────────────────────────┤          │
│          │      Input             │          │
├──────────┴────────────────────────┴──────────┤
│ Status Bar                                    │
└───────────────────────────────────────────────┘
```

The command palette and search overlay are rendered on top of the layout using a character-level overlay function.

## Own Model DTOs

Chit TUI defines its own data transfer objects in the `model` package, mirroring the server's JSON shapes. This avoids importing the server codebase (and its dependencies like `pgx`, `chi`, etc.) into the TUI.
