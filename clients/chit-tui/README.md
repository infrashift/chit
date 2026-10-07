# Chit TUI

A terminal client for the [Chit](https://github.com/infrashift/chit) messaging server, built on the [Charmbracelet](https://charm.sh) Bubble Tea stack.

Fuller documentation lives on the docs site, under [`docs/src/content/docs/tui/`](../../docs/src/content/docs/tui/).

## Features

- **Palette-driven navigation** — one overlay (`Ctrl+K`) jumps to channels and DMs, sorted by mentions, then unread, then recency; `@` finds people, `/` lists commands, `?` searches this channel and `??` every channel
- **Real-time updates** — a WebSocket connection that pings to detect dead links, reconnects with backoff, and reloads the open channel after a gap
- **Threads** — `Enter` on a post opens its thread in the main pane; replies are written in the regular input, and `/threads` lists the threads you follow
- **Editing and moderation** — edit (`e`), delete (`d d`) and pin (`p`) posts; tag them with `t` or by writing `#tags`
- **History** — older messages load as you scroll up
- **Copy** — `y` copies the selection or the selected post through OSC 52, which works over SSH
- **Mouse** — wheel scrolling, clicks on the action bar and palette, drag to select; everything also works from the keyboard
- **Themes** — eight bundled themes in dark and light pairs, following the terminal's appearance, plus your own TOML themes

## Requirements

- Go 1.25+
- A running [Chit server](https://github.com/infrashift/chit)

## Installation

```bash
# The TUI is a nested Go module in the Chit monorepo
git clone https://github.com/infrashift/chit.git
cd chit/clients/chit-tui
make build   # binary at bin/chit-tui
```

## Configuration

Settings are read from `~/.config/chit/config.toml` (set `CHIT_CONFIG_FILE` to use another path) and from the environment. An environment variable overrides the file, which overrides the built-in default. The file is checked against a schema (`internal/config/schema/config.cue`); an invalid setting is ignored with a warning rather than stopping the client.

| File key | Environment | Default | Meaning |
|---|---|---|---|
| `server_url` | `CHIT_SERVER_URL` | — (required) | Chit server or gateway URL, e.g. `http://localhost:4455` |
| — | `CHIT_SESSION_TOKEN` | — | Session token to use instead of signing in |
| `session_file` | `CHIT_SESSION_FILE` | `~/.config/chit/session.json` | Where the signed-in session is kept (written `0600`) |
| `ws_scheme` | `CHIT_WS_SCHEME` | `wss` for an `https://` server, else `ws` | WebSocket scheme |
| `auth_header` | `CHIT_AUTH_HEADER` | `X-Session-Token` | Header the session token is sent in |
| `theme` | `CHIT_THEME` | — | Theme to use whatever the terminal's appearance |
| `theme_dark`, `theme_light` | — | — | Themes for dark and light terminals, used when `theme` is not set |
| `appearance` | — | `system` | `dark`, `light`, or `system` (ask the terminal) |

```toml
# ~/.config/chit/config.toml
server_url  = "http://localhost:4455"
theme_dark  = "kanagawa"
theme_light = "kanagawa-lotus"
```

### Command-line flags

| Flag | Meaning |
|---|---|
| `--theme <name>` | Use this theme for this run; an unknown name is an error |
| `--appearance dark\|light\|system` | Override the configured appearance |
| `--list-themes` | List bundled and local themes, then exit |

## Themes

Bundled: `tokyo-night` and `tokyo-night-day` (the defaults for dark and light terminals), `catppuccin`, `catppuccin-latte`, `kanagawa`, `kanagawa-lotus`, `nightfox`, `dayfox`.

Pick one for the session with `/theme`. It is saved to the config file: as `theme`, or, if you have set `theme_dark` and `theme_light`, as the one for the current appearance, so switching keeps working.

### Your own themes

A theme is a TOML file in `~/.config/chit/themes/`, named after the theme in lowercase, e.g. `my-theme.toml` for `--theme my-theme`. Every palette key is required; colors are hex values or terminal color names (`red`, `light_blue`, …), which follow your terminal's own palette.

```toml
name   = "My Theme"
author = "you"

background      = "#1e1e2e"
foreground      = "#cdd6f4"
subtle          = "#6c7086"
accent          = "#89b4fa"
error           = "#f38ba8"
success         = "#a6e3a1"
warning         = "#f9e2af"
border          = "#313244"
active_border   = "#89b4fa"
highlight       = "#313244"
muted           = "#585b70"
username        = "#cba6f7"
timestamp       = "#6c7086"
unread_badge    = "#89b4fa"
pin_badge       = "#f9e2af"
channel_active  = "#89b4fa"
mention_badge   = "#f38ba8"
mention_text    = "#f9e2af"
mention_self_bg = "#45475a"
tag_badge       = "#94e2d5"

# Optional; derived from the palette when left out.
# selection           = "#45475a"
# search_match        = "#f9e2af"
# search_match_active = "#fab387"
```

The schema is `internal/config/schema/theme.cue`. JSON skins in the older `~/.config/chit/skins/` directory still load, with a warning to move them.

## Keys

Press `?` in the history pane for the full list. The essentials:

| Key | Action |
|---|---|
| `Ctrl+K` | Palette: jump to a channel or DM; `@` people, `/` commands, `?` search, `??` search everywhere |
| `Ctrl+S` / `Ctrl+D` / `Ctrl+N` | Search messages / find people / create a channel |
| `Tab` / `Shift+Tab` | Switch between the history pane and the input |
| `Enter` | History: open the post's thread · Input: send |
| `j` / `k` | Move between posts |
| `e` / `d d` / `p` / `t` / `y` | Edit / delete / pin / tag / copy the selected post |
| `Esc` | Close an overlay, leave a thread, or cancel an edit |
| `Ctrl+C` | Quit |

### Commands

These run in the client; any other `/command` is sent to the server, which decides what it means.

| Command | Action |
|---|---|
| `/theme` (`/skin`) | Choose a theme |
| `/group` | Start a group conversation with three or more people |
| `/nick <name>` | Change your display name |
| `/username <handle>` | Change your username (breaks existing @mentions) |
| `/threads` | Threads you follow in this team |
| `/leave` | Leave the current channel |
| `/logout` | Sign out and clear the stored session |

## Architecture

The client follows the Elm architecture (model, update, view) through [Bubble Tea](https://github.com/charmbracelet/bubbletea).

```
cmd/chit-tui/          Entry point, flags, theme resolution
internal/
├── api/               HTTP client for the Chit REST API
├── auth/              Kratos sign-in, token and session storage
├── config/            config.toml loading, CUE schemas, saving the theme
├── model/             DTOs mirroring the server's JSON
├── ws/                WebSocket client: one session per Connect, pings, backoff
├── testutil/          Shared test helpers
└── tui/               Root model, split by concern:
    ├── model.go           Model, NewModel, Init
    ├── update.go          Update: the message dispatcher
    ├── keys.go, mouse.go  Keyboard and mouse input
    ├── view.go            View, overlay layout
    ├── focus.go           Focus between panes and overlays
    ├── channels.go, unread.go, history.go, users.go, tags.go
    ├── slash.go           Client-side slash commands
    ├── wsevents.go        WebSocket events and connection state
    ├── session.go         Sign-in, sign-out, session expiry
    ├── commands.go, messages.go   tea.Cmd wrappers and tea.Msg types
    ├── actionbar/ chcreator/ dmpicker/ help/ input/ login/ mention/
    ├── palette/ post/ skinpicker/ tagpicker/ thread/ threadinbox/ viewport/
    └── ui/                listwin, markdown, styles, theme
tests/e2e/             End-to-end tests against a real server
```

## Development

```bash
make all     # lint, test, build
make test    # go test -race ./...
make cover   # coverage report
make lint    # golangci-lint
make test-e2e  # end-to-end suite in a podman pod (Postgres + chitd)
```

The functional flows in `internal/tui/flow_test.go` run the model in a real Bubble Tea program with [teatest](https://github.com/charmbracelet/x/tree/main/exp/teatest). Benchmarks for rendering live in `internal/tui/viewport/bench_test.go` and `internal/tui/bench_test.go`.
