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
It is one `Model` type, but its methods are split by concern across files in
`internal/tui` rather than kept in one large file:

| File | Concern |
|------|---------|
| `model.go` | The `Model` struct, `NewModel`, `Init`, transient status messages |
| `update.go` | `Update`: the top-level message switch |
| `keys.go` | Key routing: overlays, global bindings, then the focused pane |
| `mouse.go` | Mouse routing: wheel, clicks, drag selection |
| `slash.go` | Client-side slash commands; everything else goes to the server |
| `view.go` | `View`, the overlay table, and overlay compositing |
| `focus.go` | Focus changes and delegating keys to the focused pane |
| `channels.go` | Channel and DM bookkeeping, selecting a channel |
| `unread.go` | Unread and mention counts |
| `history.go` | Loading history, threads, editing, older pages |
| `users.go` | User lookups and mention autocomplete entries |
| `tags.go` | Tag lookups and tagging posts |
| `wsevents.go` | WebSocket events and connection state |
| `session.go` | Login, session expiry, logout |
| `clipboard.go` | Copying via OSC 52 |
| `commands.go` | `tea.Cmd` wrappers around API calls |
| `messages.go` | The `tea.Msg` types those commands return |
| `keymap.go` | The global `KeyMap` |

## Component Tree

```
Root Model (internal/tui)
├── Login           — Email/password screen, shown until signed in
├── Viewport        — Post list with scroll (main pane, channel view)
├── Thread          — Thread view (swaps into the main pane)
├── Input           — Message textarea (channel posts and thread replies)
├── Action Bar      — Bottom bar: clickable buttons, context, connection dot
├── Palette         — Unified overlay: channels/people/commands/search
├── Help            — Keybinding and mouse reference overlay
├── Mention         — @mention autocomplete popup
├── Member Picker   — Member selection for private channels and /group (dmpicker)
├── Thread Inbox    — Threads you follow (/threads)
├── Skin Picker     — Runtime theme switcher (/theme)
├── Channel Creator — New channel form
└── Tag Picker      — Post tag management overlay
```

## Message Flow

1. **User presses a key** → Bubble Tea delivers a `tea.KeyMsg`
2. **Root Update** hands it to `handleKey` (`keys.go`), which routes it to a
   visible overlay first (help, palette, pickers), then to the mention popup
3. Otherwise checks global keybindings (Tab, Ctrl+K, Ctrl+S, …)
4. If not a global key, **delegates** to the focused sub-component
5. Sub-component returns a **tea.Cmd** (async operation) if needed
6. Cmd executes and produces a **result Msg** (e.g. `PostsLoadedMsg`)
7. Root Update handles the result, updating state and triggering new Cmds

Before any of that, `Update` checks whether the message reports a request the
server rejected as unauthorized. A rejected session is the same problem
whichever request noticed it, so it is caught once: the stored session is
cleared and the login screen returns.

Mouse events (`tea.MouseMsg`) are routed by `handleMouse` (`mouse.go`): the
wheel scrolls the main pane or moves the palette cursor, press-and-drag in the
history selects lines, and clicks are hit-tested against the action bar's
button spans or the palette's rows. Every clickable action dispatches through
the same code path as its keybinding.

## The Unified Palette

The palette (`internal/tui/palette`) is the single navigation entry point,
opened with `Ctrl+K`. The first character of the query selects the mode:

| Prefix | Mode | Data source |
|--------|------|-------------|
| *(none)* | Jump to channel/DM | All-team channel list + DMs, fuzzy filtered |
| `@` | Find people | `SearchUsers` (debounced), Enter opens a DM |
| `/` | Slash commands | Client commands plus the server command list, fuzzy filtered |
| `?` | Message search | `SearchPosts`, scoped to the active channel |
| `??` | Message search | `SearchPostsEverywhere`, every channel the user belongs to |

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

Authentication is handled by `authTransport`, an `http.RoundTripper` that
injects the token into every request under the configured header
(`auth_header`, default `X-Session-Token`). Every request times out after 30
seconds, so a server that accepts a request and never answers produces an
error instead of a command that waits forever.

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
    State() <-chan ConnState
    SetToken(token string)
}
```

Each `Connect` starts one session, owned by a single goroutine that reads the
connection until it fails and then redials, until `Close` ends the session.
`Close` is safe to call twice, and `Connect` may start a new session
afterwards.

- **Dead links are detected.** The client sends a ping every 25 seconds and
  treats 60 seconds without any frame as a dead connection. Without a
  deadline, a half-open TCP connection would block the reader forever.
- **Redials back off.** Delays grow exponentially from 500ms to a 30s cap,
  with up to a fifth shaved off at random so clients dropped by the same
  server restart do not all redial in the same instant.
- **A failed first dial is retried.** `Connect` returns the error so the UI
  can show it, then keeps redialing in the background like after a drop.
- **Rejected credentials stop the loop.** A handshake refused with 401 or 403
  is reported as `ErrUnauthorized`, and the client stops redialing — retrying
  cannot fix it. The TUI responds by returning to the login screen.

Events are delivered through a buffered channel that the TUI reads via the
`ListenWebSocket` tea.Cmd; connect and disconnect transitions come on the
separate `State()` channel, read by `ListenWSState`, so server events and
transport state never have to be told apart. The action bar shows a
connection indicator (`●` connected / `○` disconnected).

Events that arrive while the socket is down are gone for good — the stream
has no replay — so on reconnecting the TUI re-reads the active channel. The
same happens if the event buffer overflows while the socket stays up: the
client reports the gap as a `Desynced` state, and the TUI reloads rather than
silently showing a stale view.

### Stale Event Protection

The root model tracks a `lastWSSeq` counter. WebSocket events with a sequence number less than or equal to the last seen sequence are dropped, preventing duplicate or out-of-order message processing.

REST responses get the same care. Requests run asynchronously, and the user
may have moved on by the time one answers, so a late response is dropped
unless it still matches what is on screen:

- channel history only if it is for the active channel
- a thread only if that thread is still open
- message or people search results only if they are for the latest query

Applying one anyway would, for example, show one channel's history under
another's name.

## Focus and Panes

The root model owns a `FocusArea` enum. Only the focused component receives
key messages. `Tab`/`Shift+Tab` toggle between the top pane (history, or the
thread when one is open) and the input.

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

In a thread the bar gains an `[esc Back]` button, and while the history pane
is focused with a post selected it gains `[↵ Reply]`, which is also the hint
that the pane has to be focused before `Enter` opens a thread.

Unread/mention state, DM display names, and the all-team channel lists are
owned by the root model; the palette and action bar render from that state.

## Own Model DTOs

Chit TUI defines its own data transfer objects in the `model` package, mirroring the server's JSON shapes. This avoids importing the server codebase (and its dependencies like `pgx`, `chi`, etc.) into the TUI.
