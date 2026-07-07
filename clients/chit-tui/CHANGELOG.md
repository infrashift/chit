# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Core TUI** — Full terminal client with Elm Architecture (Model-Update-View) using Bubble Tea
- **Sidebar** — Team and channel navigation with cursor-based selection
- **Viewport** — Scrollable post list with Glamour markdown rendering
- **Input** — Message composition with textarea, Enter to send
- **Multiline input** — Alt+Enter inserts newlines; Up/Down arrows navigate between lines
- **Thread panel** — Side panel for viewing and replying to threaded conversations
- **Thread reply count badges** — Posts display `[N replies]` badge with live WebSocket updates
- **Command palette** — Fuzzy-filter overlay for slash commands (`Ctrl+K`)
- **Search overlay** — Search posts in the current channel (`Ctrl+S`)
- **@mention autocomplete** — Type `@` in the input to trigger a popup with channel members and special mentions (`@all`, `@channel`, `@here`)
- **Mention highlighting** — `@username` references in posts are styled with theme colors; self-mentions get a distinct background
- **Sidebar mention badges** — `[@N]` badges on channels with unread mentions, cleared on channel view
- **Direct messaging** — DM picker overlay (`Ctrl+D`) with user search to create 1:1 direct message channels
- **Group channels** — Multi-select users in the DM picker (`Tab` to toggle) to create group conversations
- **Channel creation** — Channel creator overlay (`Ctrl+N`) with display name, auto-slug, type toggle (Open/Private), and purpose fields
- **Sidebar DM section** — Direct messages listed below team channels with resolved display names and unread/mention badges
- **Runtime theme switching** — `/skin` command opens a picker overlay to hot-swap themes without restarting
- **Theme-aware markdown** — Post rendering uses Glamour styles derived from the active theme, including Chroma syntax highlighting
- **WebSocket client** — Real-time event streaming with automatic reconnection and exponential backoff
- **WebSocket events** — Handlers for `posted`, `thread_updated`, `mentioned`, and `channel_created` event types
- **Unread badges** — Per-channel unread message counts computed from channel member data
- **API client** — Full HTTP client implementing `ChitClient` interface with `authTransport` for session token injection
- **Configurable auth header** — `CHIT_AUTH_HEADER` env var overrides the default `X-Session-Token` header name (used for UAT with `X-User-Id`)
- **Theme system** — Built-in themes (Tokyo Night, Catppuccin Mocha, Kanagawa, Nightfox) with JSON theme file support
- **Custom themes** — Load themes from `~/.config/chit/skins/<name>.json` via `CHIT_THEME` env var
- **Focus management** — Tab/Shift+Tab cycles between sidebar, viewport, input, and thread
- **Configuration** — Environment variable-based config (`CHIT_SERVER_URL`, `CHIT_SESSION_TOKEN`, `CHIT_WS_SCHEME`, `CHIT_THEME`)
- **Post rendering** — Glamour-powered markdown with username, timestamp, pinned badge, and encrypted badge
- **E2E testing infrastructure** — Podman-based test pipeline (`make test-e2e`) with containerized Chit server, PostgreSQL, and automated API/WebSocket tests
- **Post tagging** — Tag posts with flat string tags to organize conversations; tags displayed as `#tag` badges on posts
- **Tag picker overlay** — Press `t` in the viewport to open a tag picker; filter tags by prefix, toggle with Enter, create new tags with `Ctrl+N`
- **Hashtag input syntax** — Type `#tag` in the message input to auto-apply tags when the post is created; tags are stripped from the message content
- **Tag-based search** — Use `#tag` syntax in the search overlay (`Ctrl+S`) to filter results by tags; supports hybrid text + tag search
- **Documentation site** — Astro + Starlight static docs at `docs/`
- **Keybindings reference** — Documented `Ctrl+N` (channel creator), `Ctrl+D` (DM picker), `t` (tag picker), channel creator fields, DM picker multi-select, and tag picker controls
- **UAT tutorial scenarios** — Scenarios for creating team channels, direct messages, and group channels (Scenarios 11-13)

### Changed

- Viewport and thread renderers use theme-derived Glamour style configs instead of the built-in `"dark"` style
- UAT tutorial Scenario 2 rewritten to reflect auto-selection behavior (sidebar starts in channels view, not teams view)

### Fixed

- Sidebar not responding to keys on launch — `NewModel()` now calls `sidebar.Focus()` so the sidebar accepts input immediately without requiring a full Tab cycle
- Theme switching not updating markdown rendering — `SetStyles()` resets the Glamour renderer so posts re-render with the new theme's colors
