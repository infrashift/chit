# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **TOML themes missing from the theme list** — themes in `~/.config/chit/themes/` loaded by name but never appeared in `/theme` or `--list-themes`. Names that cannot load (not lowercase, or shadowed by a bundled theme) are no longer listed
- **`/theme` turned off appearance switching** — picking a theme saved `theme = …`, which outranks `theme_dark` and `theme_light`. With those configured, the pick is now saved for the current appearance
- **Invisible selection with named-color themes** — a theme using terminal color names got a selection and search highlight the same color as the background
- **@-mentions after accented text** — completing a mention on a line with a non-ASCII character before the `@` cut the line in the wrong place. An email address no longer opens the mention popup, and `@bob.` at the end of a sentence is highlighted as a mention of bob
- **Pickers lost the cursor** — the theme, tag and member pickers drew only their first few rows, so moving down past them moved the cursor out of sight, and the thread inbox ran off the bottom of the screen. All of them now scroll to keep the cursor visible
- **Palette chose the wrong row** — after moving down and then typing, Enter picked whatever landed at the old cursor index rather than the top match
- **New tags missing from the tag picker** — a tag created with `Ctrl+N` was applied but did not appear in the open picker
- **HTTPS servers got an insecure WebSocket** — `CHIT_WS_SCHEME` defaulted to `ws` whatever the server; it now defaults to `wss` for an `https://` server. A trailing `/` on the server URL no longer breaks every path
- **Requests could hang forever** — API requests now time out after 30s
- **Session file permissions** — an existing `session.json` kept whatever mode it had; it is now rewritten atomically, readable by the owner alone
- **Slow channel switches and thread opens** — every channel switch and thread open or close re-rendered the whole loaded history through glamour, though the pane size had not changed. Opening and closing a thread with 300 posts loaded took 113ms; it now takes about 4ms. Tags for a page of history are applied in one redraw instead of one per post, and moving the cursor no longer re-strips the whole history
- **Crash when searching** — a search highlight could crash the client on text containing characters that change length when lowercased (such as the Kelvin sign), and the post under the cursor was drawn without the highlight
- **Repeated requests** — authors the server does not return (deleted users) were asked for on every post and page load; DM member rows were refetched for every conversation on each DM event; the whole tag list was refetched after every tagging
- **Late responses overwrote the screen** — a slow history load for the channel just left replaced the one opened; likewise an earlier thread, search or user search. Each response is now dropped unless it is for what is on screen
- **Replies typed while a thread loads** went to the channel as top-level posts; they are now replies. A failed thread load returns to the channel instead of leaving an empty pane
- **Forgotten edits** — an edit started and abandoned was applied to the next message sent anywhere. Switching channel, opening a thread or `Esc` now cancels it, and the action bar shows "editing" while one is open
- **`d` deleted at once** — it now asks; press `d` again to confirm
- **Keys landing in the hidden history pane** — after the tag picker closed over a thread, or the thread's root was deleted mid-reply, single-letter keys acted on posts the reader could not see
- **Hashtags mangled messages** — sending a message with a `#tag` collapsed double spaces (breaking code indentation), joined lines, treated `#123` and `#include` in code as tags, and editing dropped the tags. Tags now leave the rest of the message as written, and land on the post that carried them
- **Thread pane** — your own reply sometimes never appeared, reply counts ran one high, and deleted replies stayed visible
- **History paging** — scrolling up while a channel loaded fetched the wrong page, and overlapping pages showed posts twice
- **Signing out kept the session** — whoever signed in next saw the previous user's channel and history. Signing out now starts from nothing; after an expiry, the same user keeps their place and the open channel is re-read, while anyone else starts fresh
- **Crash on command output** — a slash command's response crashed the client when a post's author had not loaded (a deleted user, or a failed lookup)
- **Repeated command output** — a second `/help` showed the first one's text, and output lost its `/slug` author whenever users loaded
- **Tests touched the real config** — the suite saved a theme to `~/.config/chit/config.toml` and deleted `~/.config/chit-tui/session.json`; it now runs against a temporary home
- **Real-time updates after signing back in** — signing out closed the WebSocket client for good: the next session showed "connected" but received nothing, and a second sign-out crashed the client. Each sign-in now starts a fresh session on the same listeners
- **Dead links that looked alive** — a connection left half-open by sleep or a NAT timeout stayed "connected" forever. The client now pings, treats a minute of silence as a drop, and reconnects
- **Busy redial after a long outage** — the reconnect delay overflowed to a negative wait after about 15 minutes and the client redialed in a tight loop. The delay now stays capped at 30s, with jitter
- **No real-time updates when the server was down at launch** — a failed first connection was never retried. It now retries in the background ("offline, retrying"), and rejected credentials prompt sign-in
- **One resync per overflow** — a full event buffer reported every dropped event, setting off a history reload for each
- **No post selection in channels that started empty** — the history cursor stayed unset when posts only ever arrived via WebSocket appends, so `Enter` (open thread) and `t` (tag picker) silently did nothing until the channel was reloaded; the first appended post is now selected
- **Focus loss after closing overlays** — Esc-closing the tag picker, channel creator, member picker, or skin picker left keyboard focus on the closed overlay (keys went nowhere until `Tab`/`Ctrl+K`); every overlay now restores focus to a live component on close
- **Member picker Esc abandoned the pending channel** — dismissing the member picker after submitting a private channel silently dropped it; the channel is now created without extra members (Esc only skips member selection)

### Added

- **Unified palette** — One overlay (`Ctrl+K`) for all navigation: channels/DMs sorted by mentions → unread → recency with fuzzy filtering, `@` people search (opens a DM), `/` slash commands, and `?` message search scoped to the active channel; `Ctrl+S`/`Ctrl+D` open it pre-filled
- **Action bar** — Bottom bar with clickable buttons (`^K Jump`, `^S Search`, `^D DM`, `^N New`, `? Help`, `esc Back` in threads), active team/channel context, transient errors, and a WebSocket connection indicator
- **Mouse support** — Wheel scrolls the history/thread pane and moves the palette cursor; clicks trigger bar buttons and select palette rows; clicking outside the palette closes it
- **Help overlay** — `?` (from the history pane) or the bar button shows keybindings and mouse actions
- **All-team channels** — Channels are fetched for every team at startup; palette rows show a `· team` suffix when more than one team is loaded

### Changed

- **Two-pane layout** — The persistent sidebar is gone; chat history and input span the full width, and the palette is the only channel navigation
- **Threads fill the main pane** — Enter on a post swaps the history pane for the thread; replies are composed in the regular input box and `Esc` returns to the channel (`Ctrl+T` and the side panel are removed)
- **Focus cycle** — `Tab` toggles between the main pane and the input; `Esc` in the input refocuses the history pane
- **DM picker reduced to member selection** — Starting DMs moved to the palette's `@` mode; the picker overlay now only selects members for new private channels (group-DM creation via multi-select was dropped)
- Unread/mention counts, DM display names, and per-team channel lists are owned by the root model (previously sidebar state)

### Removed

- Sidebar, command palette (`cmdpalette`), and search (`search`) components — superseded by the unified palette and action bar
- Dead code left behind by the redesign: the producer-less group-create TUI chain (`GroupCreatedMsg`, `CreateGroupChannel` command), never-produced `AuthExpiredMsg`/`LogoutMsg`, the unmatched `Enter` keybinding, unused `CreateTagCmd`/`AddChannelMemberCmd`, `Styles.Sidebar`/`Styles.Border`, and unreferenced model DTOs (`TeamMember`, `ThreadMembership`, `ThreadResponse`); `SidebarItem`/`SidebarActive` styles renamed to `ListItem`/`ListItemActive`

### Added (initial development)

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

### Changed (initial development)

- Viewport and thread renderers use theme-derived Glamour style configs instead of the built-in `"dark"` style
- UAT tutorial Scenario 2 rewritten to reflect auto-selection behavior (sidebar starts in channels view, not teams view)

### Fixed (initial development)

- Sidebar not responding to keys on launch — `NewModel()` now calls `sidebar.Focus()` so the sidebar accepts input immediately without requiring a full Tab cycle
- Theme switching not updating markdown rendering — `SetStyles()` resets the Glamour renderer so posts re-render with the new theme's colors
