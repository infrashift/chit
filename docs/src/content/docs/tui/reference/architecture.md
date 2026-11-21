---
title: Architecture
description: Technical architecture of Chit TUI.
---

Chit TUI follows the **Elm Architecture** (Model-Update-View) with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

## Elm Architecture

Every component in Chit TUI implements three functions:

- **Model** — A struct that holds all component state
- **Update** — A pure function: `(Model, Msg) → (Model, Cmd)`
- **View** — A pure function: `Model → string`

The root model composes all sub-components and routes messages to the applicable child.
It is one `Model` type. Its methods are not in one large file. Each concern
has its own file in `internal/tui`:

| File | Concern |
|------|---------|
| `model.go` | The `Model` struct, `NewModel`, `Init`, transient status messages |
| `update.go` | `Update`: the top-level message dispatcher, and the routers (`routeSessionMsg`, `routeChannelMsg`, `routePostMsg`, `routeTagMsg`, `routeLookupMsg`) that send each message to its handler |
| `keys.go` | Key routing: overlays, global bindings, then the focused pane |
| `mouse.go` | Mouse routing: wheel, clicks, drag selection |
| `slash.go` | Client-side slash commands; everything else goes to the server |
| `view.go` | `View`, the overlay table, and overlay compositing |
| `focus.go` | Focus changes and delegating keys to the focused pane |
| `channels.go` | Channel and DM bookkeeping, selecting a channel |
| `unread.go` | Unread and mention counts |
| `history.go` | Loading history, send, edit, search hits, older pages, leaving a thread |
| `threads.go` | Thread loading, the thread inbox, opening the thread of the selected post |
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
2. **Root Update** gives it to `handleKey` (`keys.go`). `handleKey` sends it
   first to a visible overlay (help, palette, pickers), then to the mention popup
3. If no overlay takes the key, `handleKey` checks the global keys (Tab, Ctrl+K, Ctrl+S, …)
4. If it is not a global key, `handleKey` **delegates** it to the focused sub-component
5. The sub-component returns a **tea.Cmd** (async operation) if necessary
6. The Cmd executes and produces a **result Msg** (e.g. `PostsLoadedMsg`)
7. Root Update handles the result. It updates the state and can start new Cmds

Before all of these steps, `Update` checks whether the message reports a
request that the server rejected as unauthorized. A rejected session is the
same problem for each request, so `Update` catches it in one place. The TUI
clears the stored session and shows the login screen again.

`handleMouse` (`mouse.go`) routes mouse events (`tea.MouseMsg`):

- The wheel scrolls the main pane or moves the palette cursor.
- Press-and-drag in the history selects lines.
- `handleMouse` hit-tests each click against the button spans of the action
  bar or the rows of the palette.

Each clickable action goes through the same code path as its key.

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
post**, so the channels with new activity are always on top. Each row shows
an unread `(N)` badge and a mention `[@N]` badge. The palette shares the root
model's unread/mention maps by reference, so the badge state needs no extra
code.

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

`authTransport` does the authentication. It is an `http.RoundTripper` that
injects the token into every request under the configured header
(`auth_header`, default `X-Session-Token`). `main` builds the client with
`api.NewClientWithTokenSource` and an `*auth.TokenStore`. The transport reads
the token from this `TokenSource` on every request, so a token that changes
after sign-in takes effect at once. Every request times out after 30 seconds. As a result, a server
that accepts a request and never answers causes an error, not a command that
waits forever.

When it starts, the TUI gets the channels of **every team** that the user
belongs to (`channelsByTeam` on the root model). If the TUI loaded more than
one team, the palette shows a `· team` suffix on channel rows.

The client also uses the routes that the HALO work added to the server:

| Area | Methods |
|------|---------|
| Tags | `GetTags` (the tags of one channel), `RenameTag`, `DeleteTag` |
| Channel tags | `GetChannelTags`, `AddChannelTag`, `RemoveChannelTag`, `FindChannelsByTags` |
| Files | `UploadFile`, `GetFileInfo`, `DownloadFile` |

`UploadFile` and `DownloadFile` read at most 10 MB, the limit of the
server. The client does not send a file that is larger.

### Server Errors

The client reads each error response into an `APIError`. The error keeps
the status, the server's error ID and message, and the `reason` and
`detail` of a membership refusal. For a `429`, it also keeps the wait from
the `Retry-After` header, to a maximum of 60 seconds.

The action bar shows `api.UserMessage(err)`. This function gives a short
message for each error that the server marks with a stable ID or status:

| Error | Message |
|-------|---------|
| `429` | `too many requests: try again in Ns` |
| `409` | `that name is in use: …` |
| `channel.membership_refused` | `the membership policy refused this: …` |
| `file.too_large`, `file.quota_exceeded` | The file is too large, or the quota is full |
| `file.svg`, `file.unsupported_type`, `file.type_mismatch` | The server accepts only PNG, JPEG, WebP and GIF images |
| `file.disabled` | Attachments are off on this server |

The TUI also checks a tag name before it sends it. `model.ValidTagName`
applies the rules of the server: 1 to 50 characters, and only letters,
digits, spaces, `-`, `_` and `.`. The server keeps the `profile/` prefix
for machines, so the TUI refuses it too.

### WebSocket Client

The `WSClient` interface gives a real-time stream of events:

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

- **The client finds dead links.** The client sends a ping every 25 seconds.
  If no frame arrives for 60 seconds, it treats the connection as dead.
  Without a deadline, a half-open TCP connection can block the reader forever.
- **Redials back off.** Delays grow exponentially from 500ms to a 30s cap.
  The client removes up to a fifth of each delay at random. As a result, the clients
  that the same server restart dropped do not all redial at the same instant.
- **The client tries a failed first dial again.** `Connect` returns the error
  so the UI can show it. Then the client continues to redial in the
  background, as after a drop.
- **Rejected credentials stop the loop.** If the server refuses a handshake
  with 401 or 403, the client reports `ErrUnauthorized` and does not redial.
  A new try cannot fix the problem. The TUI then shows the login screen.

The TUI reads events from a buffered channel through the `ListenWebSocket`
tea.Cmd. Connect and disconnect transitions come on the separate `State()`
channel, which `ListenWSState` reads. As a result, the TUI never has to separate
server events from transport state. The action bar shows a connection
indicator (`●` connected / `○` disconnected).

The TUI never gets the events that arrive while the socket is down, because
the stream has no replay. For this reason, after each reconnect, the TUI reads the active
channel again. The same occurs if the event buffer overflows while the socket
stays up. The client reports the gap as a `Desynced` state, and the TUI loads
the channel again. It does not silently show a stale view.

### Stale Response Protection

Requests run asynchronously, and the user can move to a different view before
a request answers. For this reason, the TUI drops a late REST response unless it still
matches the screen:

- channel history only if it is for the active channel
- a thread only if that thread is still open
- message or people search results only if they are for the latest query

If the TUI applied such a response, it could, for example, show the history
of one channel under the name of a different channel.

## Focus and Panes

The root model owns a `FocusArea` enum. Only the focused component receives
key messages. `Tab`/`Shift+Tab` toggle between the top pane (history, or the
thread when one is open) and the input.

A `mainPane` enum controls the top pane:

- **`paneChannel`** — the channel's post history (Viewport)
- **`paneThread`** — a thread opened with Enter on a post; the input then
  composes replies to the thread root, and `Esc` returns to the channel

Overlays (palette, help, pickers) intercept keys while they are visible.
`View` composites them over the layout in z-order through a single overlay
table. It uses the character-level, ANSI-aware `placeOverlay` function.

## Layout

`View` composes the screen with Lipgloss:

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

In a thread, the bar adds an `[esc Back]` button. When the history pane has
focus and has a selected post, the bar adds `[↵ Reply]`. This button also shows
that the pane must have focus before `Enter` opens a thread.

The root model owns the unread/mention state, the DM display names, and the
all-team channel lists. The palette and action bar render from that state.

## Own Model DTOs

Chit TUI defines its own data transfer objects in the `model` package. They mirror the server's JSON shapes. As a result, the TUI does not import the server codebase (and its dependencies like `pgx`, `chi`, etc.).
