---
title: Configuration
description: Environment variables and settings for Chit TUI.
---

Chit TUI is configured entirely via environment variables. There are no config files for the core settings.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CHIT_SERVER_URL` | Yes | — | Base URL of the Chit server or Oathkeeper proxy |
| `CHIT_SESSION_TOKEN` | No | — | Pre-set session token (skips interactive login) |
| `CHIT_SESSION_FILE` | No | `~/.config/chit-tui/session.json` | Path to the session file |
| `CHIT_WS_SCHEME` | No | `ws` | WebSocket scheme: `ws` for unencrypted, `wss` for TLS |
| `CHIT_AUTH_HEADER` | No | `X-Session-Token` | HTTP header name used for authentication |
| `CHIT_THEME` | No | — | Theme name (built-in or custom file name) |

## Required Variables

### `CHIT_SERVER_URL`

The base URL of your Chit server instance or Oathkeeper proxy. The TUI appends `/api/v1` for REST endpoints, `/api/v1/websocket` for the WebSocket connection, and `/kratos` for authentication flows.

When running behind the Ory stack, point this at the Oathkeeper proxy:

```bash
export CHIT_SERVER_URL=http://localhost:4455
```

For direct server access (bypassing auth), point at the Chit server:

```bash
export CHIT_SERVER_URL=http://localhost:8065
```

## Optional Variables

### `CHIT_SESSION_TOKEN`

A pre-set session token. When provided, the TUI skips the interactive login
screen and uses this token directly. Useful for CI/CD, automated testing, or
when you already have a valid token.

When not set, the TUI shows an interactive login screen where you enter your
email and password. After login, the session token is stored at
`~/.config/chit-tui/session.json` and reused automatically on subsequent
launches.

```bash
export CHIT_SESSION_TOKEN=abc123def456
```

### `CHIT_SESSION_FILE`

Override the path where chit-tui stores the session token. Defaults to
`~/.config/chit-tui/session.json`. Set this to run multiple instances as
different users simultaneously — each instance uses its own session file.

```bash
# Terminal A — logs in as Alice
CHIT_SESSION_FILE=/tmp/alice.json CHIT_SERVER_URL=http://localhost:4455 chit-tui

# Terminal B — logs in as Bob
CHIT_SESSION_FILE=/tmp/bob.json CHIT_SERVER_URL=http://localhost:4455 chit-tui
```

### `CHIT_WS_SCHEME`

Controls the WebSocket protocol. Use `ws` for local development or `wss` when connecting to a TLS-secured server.

```bash
export CHIT_WS_SCHEME=wss
```

### `CHIT_AUTH_HEADER`

Controls which HTTP header carries the authentication token. Defaults to `X-Session-Token` for production deployments behind an Oathkeeper proxy. Set to `X-User-Id` when connecting directly to a Chit server with a user ID instead of a session token.

```bash
export CHIT_AUTH_HEADER=X-User-Id
```

### `CHIT_THEME`

Select a built-in theme by name, or a custom theme by filename (without `.json`). See the [Themes guide](/chit/tui/guides/themes/) for details.

```bash
export CHIT_THEME=catppuccin
```

## Session Management

When `CHIT_SESSION_TOKEN` is **not** set, the TUI manages sessions
automatically:

1. On first launch, a login screen prompts for email and password
2. Credentials are submitted to Kratos via the Oathkeeper proxy
3. The session token is stored at `~/.config/chit-tui/session.json` (permissions `0600`)
4. On subsequent launches, the stored token is validated; if still valid, login
   is skipped
5. If the token has expired or is invalid, the login screen reappears
6. Use `/logout` in the TUI to clear the session and return to the login screen

## Example Shell Configuration

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
# Chit TUI — production (interactive login)
export CHIT_SERVER_URL=https://chat.example.com
export CHIT_WS_SCHEME=wss
export CHIT_THEME=kanagawa
```
