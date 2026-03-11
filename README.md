# Chit TUI

A terminal user interface for the [Chit](https://github.com/infrashift/chit) messaging server. Built with Go and the [Charmbracelet](https://charm.sh) Bubble Tea ecosystem.

## Features

- **Dual-pane navigation** — Sidebar with team/channel tree, main viewport for messages
- **Real-time messaging** — WebSocket connectivity with automatic reconnection and exponential backoff
- **Threaded conversations** — Side panel for viewing and replying to threads
- **Markdown rendering** — Posts rendered with [Glamour](https://github.com/charmbracelet/glamour) for rich terminal output
- **Command palette** — Fuzzy-filter overlay for slash commands (`Ctrl+K`)
- **Post search** — Search posts in the current channel (`Ctrl+S`)
- **Unread badges** — Per-channel unread message counts in the sidebar
- **Customizable themes** — Built-in themes (Tokyo Night, Catppuccin, Kanagawa, Nightfox) and JSON theme file support

## Requirements

- Go 1.24+
- A running [Chit server](https://github.com/infrashift/chit) instance
- A valid session token

## Installation

```bash
# Clone and build
git clone https://github.com/infrashift/chit-tui.git
cd chit-tui
make build

# Binary is at bin/chit-tui
```

## Configuration

Chit TUI is configured via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CHIT_SERVER_URL` | Yes | — | Chit server URL (e.g. `http://localhost:8065`) |
| `CHIT_SESSION_TOKEN` | Yes | — | Session token for authentication |
| `CHIT_WS_SCHEME` | No | `ws` | WebSocket scheme (`ws` or `wss`) |
| `CHIT_THEME` | No | — | Theme name (`catppuccin`, `kanagawa`, `nightfox`, `tokyo-night`) |

```bash
export CHIT_SERVER_URL=http://localhost:8065
export CHIT_SESSION_TOKEN=your-session-token
export CHIT_THEME=catppuccin

./bin/chit-tui
```

## Keybindings

| Key | Action |
|-----|--------|
| `Tab` | Next pane |
| `Shift+Tab` | Previous pane |
| `Ctrl+T` | Toggle thread panel |
| `Ctrl+K` | Open command palette |
| `Ctrl+S` | Open search |
| `Esc` | Close overlay / thread |
| `Enter` | Send message / select item |
| `Ctrl+C` | Quit |

Within the sidebar, use `j`/`k` or arrow keys to navigate teams and channels. Press `Enter` to select, `Esc`/`Backspace` to go back to teams.

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
    ├── messages.go            All tea.Msg types
    ├── commands.go            tea.Cmd wrappers for API calls
    ├── keymap.go              Global keybindings
    ├── sidebar/               Team/channel navigation
    ├── viewport/              Post list viewport
    ├── input/                 Message input textarea
    ├── thread/                Thread side panel
    ├── post/                  Single post renderer (Glamour markdown)
    ├── cmdpalette/            Command palette overlay
    ├── search/                Search overlay
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
