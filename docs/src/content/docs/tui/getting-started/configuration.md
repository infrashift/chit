---
title: Configuration
description: Config file, environment variables, and command-line flags for Chit TUI.
---

Chit TUI reads its settings from a TOML config file and from environment
variables. An environment variable overrides the file, and the file overrides
the built-in default. The file is the persistent choice. An environment
variable is a deliberate override for one run, so it wins.

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

The client checks the file against a CUE schema
(`clients/chit-tui/internal/config/schema/config.cue`). The client drops a key
that the schema does not know, or a value that the schema rejects (for example
`ws_scheme = "http"`). It writes a warning on stderr, and that setting falls
back to its default. The client ignores a file that is not valid TOML
completely, again with a warning.

None of these problems stop the client when it starts. A typo in a config file
must not lock you out of the chat client. For this reason, the only fatal
configuration error is an absent server URL.

## Settings

| Config key | Environment variable | Default | Description |
|------------|----------------------|---------|-------------|
| `server_url` | `CHIT_SERVER_URL` | — (required) | Base URL of the Chit server or Oathkeeper proxy |
| `ws_scheme` | `CHIT_WS_SCHEME` | `wss` for an `https://` server, else `ws` | WebSocket scheme: `ws` or `wss` |
| `auth_header` | `CHIT_AUTH_HEADER` | `X-Session-Token` | HTTP header that carries the token |
| `session_file` | `CHIT_SESSION_FILE` | `~/.config/chit/session.json` | Where the login session is stored |
| `theme` | `CHIT_THEME` | — | Theme to use regardless of appearance |
| `theme_dark`, `theme_light` | — | — | Themes for dark and light terminals, used when `theme` is not set |
| `appearance` | — | `system` | `dark`, `light`, or `system` (ask the terminal) |
| `theme_watch` | — | `true` | Apply each change to the active local theme file while the client runs |
| `theme_paint` | — | `true` | Set the default colors of the terminal to the colors of the theme |
| `download_dir` | `CHIT_DOWNLOAD_DIR` | the XDG download directory, else `~/Downloads` | Where `s` in the post details saves the files of a post |
| — | `CHIT_SESSION_TOKEN` | — | Pre-set session token (skips interactive login) |

`CHIT_SESSION_TOKEN` has no config-file key; it can only come from the
environment.

### `server_url` / `CHIT_SERVER_URL`

The base URL of the Chit server instance or Oathkeeper proxy. The TUI adds
`/api/v1` for REST endpoints, `/api/v1/websocket` for the WebSocket
connection, and `/kratos` for authentication flows. The TUI removes a slash at
the end, so `http://localhost:4455/` works the same as `http://localhost:4455`.

When the server runs behind the Ory stack, point this at the Oathkeeper proxy:

```bash
export CHIT_SERVER_URL=http://localhost:4455
```

For direct server access without auth, point at the Chit server:

```bash
export CHIT_SERVER_URL=http://localhost:8065
```

If neither the environment nor the config file sets it, `chit-tui` exits with
a `config error` that names both places.

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
deployments behind an Oathkeeper proxy. Set it to `X-User-Id` when you connect
directly to a Chit server with a user ID instead of a session token.

```bash
export CHIT_AUTH_HEADER=X-User-Id
```

### `session_file` / `CHIT_SESSION_FILE`

Override the path where chit-tui stores the session token. Defaults to
`~/.config/chit/session.json`. Set this to run multiple instances as
different users simultaneously — each instance uses its own session file.

```bash
# Terminal A — logs in as Alice
CHIT_SESSION_FILE=/tmp/alice.json CHIT_SERVER_URL=http://localhost:4455 chit-tui

# Terminal B — logs in as Bob
CHIT_SESSION_FILE=/tmp/bob.json CHIT_SERVER_URL=http://localhost:4455 chit-tui
```

### `CHIT_SESSION_TOKEN`

A pre-set session token. When provided, the TUI skips the interactive login
screen and uses this token directly. It does not check the token first. Useful for
CI/CD, automated testing, or when you already have a valid token.

When this variable is not set, the TUI shows an interactive login screen. On
that screen, you enter the email and password. After login, the TUI stores the
session token in the session file. The TUI uses it again automatically when
it starts the next time.

```bash
export CHIT_SESSION_TOKEN=abc123def456
```

### `theme`, `theme_dark`, `theme_light`, `appearance`

`theme` (or `CHIT_THEME`) names one theme to use whatever the terminal looks
like. To follow the terminal, leave `theme` unset and set `theme_dark` and
`theme_light`. Then `appearance` decides which of the two applies. The value
`system` (the default) asks the terminal whether its background is dark.

```toml
theme_dark  = "tokyo-night"
theme_light = "tokyo-night-day"
```

A configured theme name that does not resolve is a warning, not an error: the
client falls through to the next choice. See the
[Themes guide](/chit/tui/guides/themes/) for the full order and for how
`/theme` saves the choice of the user back to this file.

### `theme_watch`, `theme_paint`

Both keys are TOML booleans, and both are `true` when the file does not set
them. Another type, such as `"no"`, is a warning, and the client uses the
default.

- `theme_watch` applies each saved change to the file of the active local
  theme while the client runs.
- `theme_paint` sets the default foreground and background of the terminal
  to the colors of the theme. The client gives the terminal its own colors
  back when it stops.

```toml
theme_watch = true
theme_paint = false
```

See [Change the Theme While the Client Runs](/chit/tui/guides/themes/#change-the-theme-while-the-client-runs).

## Command-Line Flags

Flags override everything above for a single run:

| Flag | Effect |
|------|--------|
| `--theme <name>` | Use this theme; an unknown name is an error |
| `--appearance dark\|light\|system` | Override the configured appearance |
| `--list-themes` | Print the bundled and local themes, then exit |

A misspelled `--theme` stops the client, but a misspelled `theme` in the
config file only gives a warning. A person who typed a name a moment before
wants to know that the name is wrong. A config file that is out of date must
not stop the client when it starts.

## Session Management

When `CHIT_SESSION_TOKEN` is **not** set, the TUI manages sessions
automatically:

1. On first launch, a login screen asks for email and password
2. The TUI sends the credentials to Kratos at `<server_url>/kratos`
3. The TUI stores the session token in the session file (permissions `0600`).
   The write is atomic, so an interrupted write cannot leave a truncated file
4. On later launches, the TUI validates the stored token. If the token is
   still valid, the TUI does not show the login screen
5. If the token has expired or is invalid, the login screen appears again.
   The same occurs during a session. If the server rejects the session on a
   request or on the WebSocket, the TUI clears the stored session. Then the
   TUI asks you to sign in again
6. Use `/logout` in the TUI to clear the session and return to the login screen

Every REST request times out after 30 seconds, so a server that accepts a
request and never answers produces an error rather than a hang.

## Example

A config file for a live server, with themes that follow the terminal:

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
