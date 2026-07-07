# Chit TUI

A terminal user interface for the [Chit](https://github.com/infrashift/chit) messaging server. Built with Go and the [Charmbracelet](https://charm.sh) Bubble Tea ecosystem.

## Features

- **Two-pane layout** — Chat history over the message input, with a clickable action bar along the bottom
- **Unified palette** — One overlay (`Ctrl+K`) for everything: jump to channels/DMs sorted by unread activity, `@` people search, `/` slash commands, `?` message search
- **Real-time messaging** — WebSocket connectivity with automatic reconnection and exponential backoff
- **Threaded conversations** — Enter on a post opens its thread in the main pane; reply from the regular input, `Esc` returns
- **Mouse support** — Wheel-scroll history, click action-bar buttons and palette rows (everything stays keyboard-drivable)
- **Markdown rendering** — Posts rendered with [Glamour](https://github.com/charmbracelet/glamour) for rich terminal output
- **Unread badges** — Per-channel unread and mention counts surface in the palette, which sorts active channels to the top
- **Customizable themes** — Built-in themes (Tokyo Night, Catppuccin, Kanagawa, Nightfox) and JSON theme file support

## Requirements

- Go 1.24+
- A running [Chit server](https://github.com/infrashift/chit) instance

## Installation

```bash
# Clone the Chit monorepo and build the TUI (nested Go module)
git clone https://github.com/infrashift/chit.git
cd chit/clients/chit-tui
make build

# Binary is at bin/chit-tui
```

## Configuration

Chit TUI is configured via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CHIT_SERVER_URL` | Yes | — | Chit server or Oathkeeper proxy URL (e.g. `http://localhost:4455`) |
| `CHIT_SESSION_TOKEN` | No | — | Pre-set session token (skips the interactive login screen) |
| `CHIT_SESSION_FILE` | No | `~/.config/chit-tui/session.json` | Path to the persisted session file |
| `CHIT_WS_SCHEME` | No | `ws` | WebSocket scheme (`ws` or `wss`) |
| `CHIT_AUTH_HEADER` | No | `X-Session-Token` | HTTP header name used for authentication |
| `CHIT_THEME` | No | — | Theme name (`catppuccin`, `kanagawa`, `nightfox`, `tokyo-night`) |

```bash
export CHIT_SERVER_URL=http://localhost:4455
export CHIT_THEME=catppuccin

./bin/chit-tui
# Log in on the interactive login screen (or pre-set CHIT_SESSION_TOKEN)
```

## Keybindings

| Key | Action |
|-----|--------|
| `Ctrl+K` | Open the palette (jump to channels/DMs; `@` people, `/` commands, `?` search) |
| `Ctrl+S` | Palette in message-search mode (`?`) |
| `Ctrl+D` | Palette in people mode (`@`) |
| `Ctrl+N` | Create a channel |
| `Tab` / `Shift+Tab` | Toggle focus between the main pane and the input |
| `Enter` | History pane: open thread · Input: send |
| `t` | Tag the selected post (history pane) |
| `?` | Help overlay (history pane) |
| `Esc` | Close overlay / leave thread / focus history pane |
| `Ctrl+C` | Quit |

In the history pane, `j`/`k` or arrow keys move the post selection. The mouse works too: wheel-scroll the history, click action-bar buttons and palette rows.

## Themes

### Built-in Themes

Set `CHIT_THEME` to one of:

- `tokyo-night` — Default. Cool blue tones with purple accents
- `catppuccin` — Catppuccin Mocha. Pastel palette with lavender and mauve
- `kanagawa` — Warm Japanese ink colors inspired by Kanagawa.nvim
- `nightfox` — Deep navy with steel blue accents from Nightfox.nvim

### Custom Themes

Create a JSON file in `~/.config/chit/skins/`:

```json
{
  "name": "My Theme",
  "author": "you",
  "background": "#1e1e2e",
  "foreground": "#cdd6f4",
  "subtle": "#6c7086",
  "accent": "#89b4fa",
  "error": "#f38ba8",
  "success": "#a6e3a1",
  "warning": "#f9e2af",
  "border": "#313244",
  "active_border": "#89b4fa",
  "highlight": "#313244",
  "muted": "#585b70",
  "username": "#cba6f7",
  "timestamp": "#6c7086",
  "unread_badge": "#89b4fa",
  "pin_badge": "#f9e2af",
  "channel_active": "#89b4fa"
}
```

Then set `CHIT_THEME=my-theme` (filename without `.json`). Any omitted color fields fall back to Tokyo Night defaults.

## Architecture

Chit TUI follows the [Elm Architecture](https://guide.elm-lang.org/architecture/) (Model-Update-View) using [Bubble Tea](https://github.com/charmbracelet/bubbletea):

```
cmd/chit-tui/main.go          Entry point
internal/
├── config/                    Environment-based configuration
├── model/                     DTOs mirroring the Chit server JSON API
├── api/                       HTTP client (ChitClient interface)
├── ws/                        WebSocket client with auto-reconnect
├── testutil/                  Test factories and mock server helpers
└── tui/
    ├── app.go                 Root model — composes all components
    ├── mouse.go               Mouse routing (wheel, action bar, palette)
    ├── messages.go            All tea.Msg types
    ├── commands.go            tea.Cmd wrappers for API calls
    ├── keymap.go              Global keybindings
    ├── viewport/              Post list (main pane, channel view)
    ├── thread/                Thread view (swaps into the main pane)
    ├── input/                 Message input textarea
    ├── palette/               Unified palette overlay (channels/people/commands/search)
    ├── actionbar/             Bottom action/status bar
    ├── help/                  Keybinding help overlay
    ├── post/                  Single post renderer (Glamour markdown)
    └── ui/
        ├── theme/             Theme definitions and JSON loader
        └── styles/            Lipgloss style constructors
```

## Development

```bash
make all        # lint + test + build
make build      # build binary
make test       # run tests
make cover      # test coverage report
make lint       # golangci-lint
make clean      # remove build artifacts
```

## License

See [LICENSE](LICENSE) for details.
