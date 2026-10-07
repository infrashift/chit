---
title: Configuration
description: Config file, environment variables, and command-line flags for Chit TUI.
---

Chit TUI reads its settings from a TOML config file and from environment
variables. An environment variable overrides the file, and the file overrides
the built-in default. The file is the persistent choice; an environment
variable is a deliberate override for one invocation, so it wins.

## Config File

The config file lives at:

```
~/.config/chit/config.toml
```

(More precisely `$XDG_CONFIG_HOME/chit/config.toml`.) Set `CHIT_CONFIG_FILE`
to read a different file instead. `chit-tui -h` prints the path in use.

Every key is optional:

```toml
# ~/.config/chit/config.toml
server_url  = "https://chat.example.com"
theme_dark  = "kanagawa"
theme_light = "kanagawa-lotus"
```

### Validation

The file is checked against a CUE schema
(`clients/chit-tui/internal/config/schema/config.cue`). A key the schema does
not know, or a value it rejects (say `ws_scheme = "http"`), is dropped with a
warning on stderr and that setting falls back to its default. A file that is
not valid TOML is ignored entirely, again with a warning.

None of this stops the client from starting. A typo in a config file should
not lock you out of your chat client, so the only fatal configuration error is
having no server URL at all.

## Settings

| Config key | Environment variable | Default | Description |
|------------|----------------------|---------|-------------|
| `server_url` | `CHIT_SERVER_URL` | — (required) | Base URL of the Chit server or Oathkeeper proxy |
| `ws_scheme` | `CHIT_WS_SCHEME` | `wss` for an `https://` server, else `ws` | WebSocket scheme: `ws` or `wss` |
| `auth_header` | `CHIT_AUTH_HEADER` | `X-Session-Token` | HTTP header that carries the token |
| `session_file` | `CHIT_SESSION_FILE` | `~/.config/chit-tui/session.json` | Where the login session is stored |
| `theme` | `CHIT_THEME` | — | Theme to use regardless of appearance |
| `theme_dark`, `theme_light` | — | — | Themes for dark and light terminals, used when `theme` is not set |
| `appearance` | — | `system` | `dark`, `light`, or `system` (ask the terminal) |
| — | `CHIT_SESSION_TOKEN` | — | Pre-set session token (skips interactive login) |

`CHIT_SESSION_TOKEN` has no config-file key; it can only come from the
environment.

### `server_url` / `CHIT_SERVER_URL`

The base URL of your Chit server instance or Oathkeeper proxy. The TUI appends
`/api/v1` for REST endpoints, `/api/v1/websocket` for the WebSocket
connection, and `/kratos` for authentication flows. A trailing slash is
trimmed, so `http://localhost:4455/` works the same as `http://localhost:4455`.

When running behind the Ory stack, point this at the Oathkeeper proxy:

```bash
export CHIT_SERVER_URL=http://localhost:4455
```

For direct server access (bypassing auth), point at the Chit server:

```bash
export CHIT_SERVER_URL=http://localhost:8065
```

If neither the environment nor the config file sets it, `chit-tui` exits with
a `config error` naming both places.

### `ws_scheme` / `CHIT_WS_SCHEME`

Controls the WebSocket protocol. You rarely need to set it: the default
follows the server URL, `wss` for `https://` and `ws` otherwise. A fixed `ws`
either failed to connect to an HTTPS server or sent the session token in the
clear. Setting it overrides that choice.

```bash
export CHIT_WS_SCHEME=wss
```

### `auth_header` / `CHIT_AUTH_HEADER`

Controls which HTTP header carries the authentication token, on both REST
requests and the WebSocket handshake. Defaults to `X-Session-Token` for
deployments behind an Oathkeeper proxy. Set to `X-User-Id` when connecting
directly to a Chit server with a user ID instead of a session token.

```bash
export CHIT_AUTH_HEADER=X-User-Id
```

### `session_file` / `CHIT_SESSION_FILE`

Override the path where chit-tui stores the session token. Defaults to
`~/.config/chit-tui/session.json`. Set this to run multiple instances as
different users simultaneously — each instance uses its own session file.

```bash
# Terminal A — logs in as Alice
CHIT_SESSION_FILE=/tmp/alice.json CHIT_SERVER_URL=http://localhost:4455 chit-tui

# Terminal B — logs in as Bob
CHIT_SESSION_FILE=/tmp/bob.json CHIT_SERVER_URL=http://localhost:4455 chit-tui
```

### `CHIT_SESSION_TOKEN`

A pre-set session token. When provided, the TUI skips the interactive login
screen and uses this token directly, without checking it first. Useful for
CI/CD, automated testing, or when you already have a valid token.

When not set, the TUI shows an interactive login screen where you enter your
email and password. After login, the session token is stored in the session
file and reused automatically on subsequent launches.

```bash
export CHIT_SESSION_TOKEN=abc123def456
```

### `theme`, `theme_dark`, `theme_light`, `appearance`

`theme` (or `CHIT_THEME`) names one theme to use whatever the terminal looks
like. Leave it unset and set `theme_dark` and `theme_light` instead to follow
the terminal: `appearance` decides which of the two applies, and `system` (the
default) asks the terminal whether its background is dark.

```toml
theme_dark  = "tokyo-night"
theme_light = "tokyo-night-day"
```

A configured theme name that does not resolve is a warning, not an error: the
client falls through to the next choice. See the
[Themes guide](/chit/tui/guides/themes/) for the full order and for how
`/theme` saves your pick back to this file.

## Command-Line Flags

Flags override everything above for a single run:

| Flag | Effect |
|------|--------|
| `--theme <name>` | Use this theme; an unknown name is an error |
| `--appearance dark\|light\|system` | Override the configured appearance |
| `--list-themes` | Print the bundled and local themes, then exit |

A misspelled `--theme` stops the client, while a misspelled `theme` in the
config file only warns. Someone who just typed a name wants to know it was
wrong; a config file that has aged out of date should not stop the client
from starting.

## Session Management

When `CHIT_SESSION_TOKEN` is **not** set, the TUI manages sessions
automatically:

1. On first launch, a login screen prompts for email and password
2. Credentials are submitted to Kratos at `<server_url>/kratos`
3. The session token is stored in the session file (permissions `0600`,
   written atomically so an interrupted write cannot leave a truncated file)
4. On subsequent launches, the stored token is validated; if still valid, login
   is skipped
5. If the token has expired or is invalid, the login screen reappears. The
   same happens mid-session: when the server rejects the session on any
   request or on the WebSocket, the stored session is cleared and you are
   asked to sign in again
6. Use `/logout` in the TUI to clear the session and return to the login screen

Every REST request times out after 30 seconds, so a server that accepts a
request and never answers produces an error rather than a hang.

## Example

A config file for a production server, with themes that follow the terminal:

```toml
# ~/.config/chit/config.toml
server_url  = "https://chat.example.com"   # ws_scheme defaults to wss
theme_dark  = "kanagawa"
theme_light = "kanagawa-lotus"
```

The same with environment variables, in `~/.bashrc` or `~/.zshrc`:

```bash
# Chit TUI — production (interactive login)
export CHIT_SERVER_URL=https://chat.example.com
export CHIT_THEME=kanagawa
```
