---
title: User Acceptance Testing
description: Test the Chit TUI application and its API clients against a real Chit server using Podman.
---

This guide covers two complementary approaches to user acceptance testing:

1. **Interactive TUI testing** — Launch the full Chit TUI application against a
   live UAT server and manually verify every feature, including interactive login.
2. **Automated e2e tests** — Run the automated test suite that validates the
   TUI's API and WebSocket clients programmatically.

Both approaches use the same Podman-based UAT environment: a containerized Chit
server with the full Ory stack (Kratos, Oathkeeper, Keto), PostgreSQL, and
pre-seeded test users, a team, and a channel.

## Prerequisites

Before you begin, make sure you have:

- **Podman** installed and available on your `PATH`
- **Go 1.24+** installed
- The **Chit server** repository cloned as a sibling directory:

```
parent/
├── chit/          # github.com/infrashift/chit
└── chit-tui/      # github.com/infrashift/chit-tui  (you are here)
```

If your Chit server repo lives somewhere else, override `CHIT_SRC`:

```bash
make test-e2e CHIT_SRC=/path/to/chit
```

## UAT Seed Data

The Chit server's seed scripts populate the database with users, a team, and a
channel. After starting the UAT pod, `make uat-seed-kratos` creates Kratos
identities so users can log in interactively via chit-tui.

### Users

All users share the password **`Password1!`**. Log in using their **email** as
the identifier.

| Username | Email                | Display Name   | Role          | Password     |
|----------|----------------------|----------------|---------------|--------------|
| alice    | alice@example.com    | Alice Anderson | `system_admin`| `Password1!` |
| bob      | bob@example.com      | Bob Baker      | `system_user` | `Password1!` |
| chad     | chad@example.com     | Chad Cooper    | `system_user` | `Password1!` |
| diana    | diana@example.com    | Diana Drake    | `system_user` | `Password1!` |
| eve      | eve@example.com      | Eve Ellis      | `system_user` | `Password1!` |

For direct API testing (bypassing auth), you can still use the `X-User-Id`
header against port 8065. See [Direct API access](#direct-api-access-bypassing-auth).

### Team and Channel

| Resource     | Name          | Display Name |
|--------------|---------------|--------------|
| Team         | `uat-team`    | UAT Team     |
| Channel      | `town-square` | Town Square  |

All five users are members of both the team and the channel.

## Authenticating as a Test User

There are two ways to authenticate against the UAT environment:

### Interactive login (chit-tui)

Point chit-tui at the **Oathkeeper proxy** (port 4455). When no
`CHIT_SESSION_TOKEN` is set, chit-tui shows a login screen. Enter any user's
**email** and **password** from the table above.

```bash
CHIT_SERVER_URL=http://localhost:4455 ./bin/chit-tui
```

After login, the session token is stored at `~/.config/chit-tui/session.json`
(or the path set by `CHIT_SESSION_FILE`) and reused on subsequent launches. To
force a fresh login:

```bash
rm ~/.config/chit-tui/session.json
```

Or use the `/logout` slash command inside chit-tui.

To run multiple instances as different users, give each its own session file:

```bash
CHIT_SESSION_FILE=/tmp/alice-session.json CHIT_SERVER_URL=http://localhost:4455 ./bin/chit-tui
```

### Direct API access (bypassing auth)

For curl testing, you can hit the Chit server directly on port 8065 with the
`X-User-Id` header, bypassing Oathkeeper entirely:

```bash
# As Alice
curl -H 'X-User-Id: a11ce000-0000-4000-a000-000000000001' \
  http://localhost:8065/api/v1/users/me

# As Bob
curl -H 'X-User-Id: b0b00000-0000-4000-a000-000000000002' \
  http://localhost:8065/api/v1/users/me
```

### Authenticated curl via Oathkeeper

To test the full auth flow with curl, obtain a session token from Kratos:

```bash
# 1. Init a login flow
FLOW=$(curl -s http://localhost:4455/kratos/self-service/login/api | jq -r .id)

# 2. Submit credentials
TOKEN=$(curl -s -X POST \
  -H 'Content-Type: application/json' \
  -d '{"method":"password","identifier":"alice@example.com","password":"Password1!"}' \
  "http://localhost:4455/kratos/self-service/login?flow=$FLOW" \
  | jq -r .session_token)

# 3. Make authenticated requests through Oathkeeper
curl -H "X-Session-Token: $TOKEN" http://localhost:4455/api/v1/users/me
```

### In Go test code

The `newTestClient` helper constructs an API client with the `X-User-Id`
header for programmatic testing:

```go
client := newTestClient(aliceKratosID)
```

For WebSocket connections, `newTestWSClient` does the same:

```go
ws := newTestWSClient(aliceKratosID, 64)
```

---

## Part 1: Interactive TUI Testing

This section walks through launching the full Chit TUI application against the
live UAT server and manually exercising every feature.

### Step 1: Start the UAT Server

From the **Chit server** repository, start the UAT environment:

```bash
cd ../chit
make uat-up
```

This creates Podman pods with PostgreSQL, the Chit server, and the full Ory
stack (Kratos, Oathkeeper, Keto, Vault, Zincsearch). Wait for services to be
ready:

```bash
podman logs -f chit-uat-app-chitd
```

Once you see the server is listening, verify:

```bash
# Direct server access
curl -sf http://localhost:8065/api/v1/system/ping

# Through Oathkeeper proxy
curl -sf http://localhost:4455/api/v1/system/ping
```

Then seed Kratos identities so users can log in:

```bash
make uat-seed-kratos
```

This creates password-based identities in Kratos for all 5 test users. It is
idempotent and safe to run multiple times.

### Step 2: Build and Launch Chit TUI

From `clients/chit-tui` in the Chit repository, build the binary and launch
it pointing at the Oathkeeper proxy:

```bash
cd clients/chit-tui
make build

CHIT_SERVER_URL=http://localhost:4455 ./bin/chit-tui
```

The TUI opens with a **login screen**. Enter a user's email and password from
the [Users table](#users) (e.g., `alice@example.com` / `Password1!`).

After login, the session token is stored locally and reused on subsequent
launches. To switch users, either use `/logout` inside the TUI, or launch with
a separate `CHIT_SESSION_FILE`:

```bash
CHIT_SESSION_FILE=/tmp/bob-session.json \
CHIT_SERVER_URL=http://localhost:4455 ./bin/chit-tui
```

:::tip[Two-terminal testing]
Run two terminals side-by-side — one as Alice and one as Bob — to verify
real-time features like WebSocket message delivery. Use `CHIT_SESSION_FILE` to
give each terminal its own session file so they don't overwrite each other.

```bash
# Terminal A (Alice)
CHIT_SERVER_URL=http://localhost:4455 \
CHIT_SESSION_FILE=/tmp/alice-session.json \
./bin/chit-tui
# Log in as alice@example.com / Password1!

# Terminal B (Bob)
CHIT_SERVER_URL=http://localhost:4455 \
CHIT_SESSION_FILE=/tmp/bob-session.json \
./bin/chit-tui
# Log in as bob@example.com / Password1!
```
:::

:::note[CI / headless mode]
For automated testing or scripts, you can still bypass the login screen by
setting `CHIT_SESSION_TOKEN` and `CHIT_AUTH_HEADER` directly against port 8065:

```bash
CHIT_SERVER_URL=http://localhost:8065 \
CHIT_SESSION_TOKEN=a11ce000-0000-4000-a000-000000000001 \
CHIT_AUTH_HEADER=X-User-Id \
./bin/chit-tui
```
:::

### Step 3: Manual Test Scenarios

Work through each scenario below. Every scenario assumes you are starting from
the main TUI screen with Town Square selected.

#### Scenario 1: Login and Verify Identity

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Launch with `CHIT_SERVER_URL=http://localhost:4455 ./bin/chit-tui` | Login screen appears with email and password fields |
| 2 | Enter `alice@example.com` and `Password1!`, press `Enter` | Login succeeds; the action bar shows **UAT Team > Town Square** |
| 3 | Observe the action bar | Your username **alice** is shown next to the connection indicator `●` |

**What this validates:** Kratos login flow (init flow, submit credentials),
Oathkeeper session validation, user profile retrieval
(`GET /api/v1/users/me`), team listing (`GET /api/v1/users/me/teams`), channel
listing (`GET /api/v1/users/me/teams/{teamId}/channels`), session persistence.

#### Scenario 2: Navigate Channels via the Palette

The TUI auto-selects the first channel on launch. All channel navigation goes
through the palette (`Ctrl+K`), which lists every channel and DM across all
your teams, sorted by mentions, then unread count, then recency.

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Press `Ctrl+K` | The palette opens listing **Town Square** |
| 2 | Type `tow` | The list fuzzy-filters to **Town Square** |
| 3 | Press `Enter` | The palette closes; the history pane loads Town Square's posts; the action bar shows **UAT Team > Town Square** |
| 4 | Press `Ctrl+K`, then `Esc` | The palette closes without changing channels |

If the user belongs to more than one team, palette rows show a `· team`
suffix so channels with the same name are distinguishable.

**What this validates:** Palette navigation, fuzzy filtering, channel
selection, all-team channel loading.

#### Scenario 3: Send a Message

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to **Town Square** | Viewport is showing the channel |
| 2 | Press `Tab` if the input is not focused | Cursor appears in the input box |
| 3 | Type `Hello from UAT!` and press `Enter` | Message appears in the viewport |
| 4 | Scroll the viewport (`k`/`j`) | The new post is visible with your username and timestamp |

**What this validates:** Post creation (`POST /api/v1/posts`), post rendering,
viewport scrolling.

#### Scenario 4: Multiline Messages

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Focus the input area (`Tab` toggles history ↔ input) | Input box is active |
| 2 | Type `First line` | Text appears in the input |
| 3 | Press `Alt+Enter` | Cursor moves to a new line; message is **not** sent |
| 4 | Type `Second line` | Input now contains two lines |
| 5 | Press `Up` arrow | Cursor moves back to the first line |
| 6 | Press `Down` arrow | Cursor returns to the second line |
| 7 | Press `Enter` | Message is sent with both lines preserved |
| 8 | Observe the viewport | The post renders with a line break between "First line" and "Second line" |

:::tip[Input Navigation]
The input area supports full multiline editing:

- **Alt+Enter** — insert a new line
- **Up/Down arrows** — move cursor between lines
- **Left/Right arrows** — move cursor within a line
- **Home/End** — jump to start/end of line
- **Enter** — send the message
:::

**What this validates:** Multiline input via `Alt+Enter`, cursor navigation
within the textarea, multiline content preserved through send.

#### Scenario 5: Real-Time Message Delivery

This scenario requires two terminals. Use `CHIT_SESSION_FILE` so each terminal
has its own session file:

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | In terminal A, launch with `CHIT_SESSION_FILE=/tmp/alice-session.json` and log in as Alice | Alice sees Town Square |
| 2 | In terminal B, launch with `CHIT_SESSION_FILE=/tmp/bob-session.json` and log in as Bob | Bob sees Town Square |
| 3 | In terminal B (Bob), send a message: `Hi Alice!` | Message appears in Bob's viewport |
| 4 | Switch to terminal A (Alice) | Alice's viewport shows Bob's message in real time |

**What this validates:** WebSocket event delivery (`ws://localhost:4455/api/v1/websocket`),
the `posted` event type, live viewport updates without manual refresh.

#### Scenario 6: Threaded Replies

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Focus the history pane (`Tab`) and navigate to a post with `j`/`k` | A post is highlighted |
| 2 | Press `Enter` | The thread fills the main pane; the action bar shows an **[esc Back]** button |
| 3 | Type a reply in the regular input and press `Enter` | Reply appears in the thread, indented under the root |
| 4 | Press `Esc` | The channel history returns; the root post shows a `[1 replies]` badge |

**What this validates:** Thread retrieval (`GET /api/v1/posts/{postId}/thread`),
reply creation (`POST /api/v1/posts` with `root_id`), the thread main-pane view.

#### Scenario 7: Search

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Press `Ctrl+S` | The palette opens in search mode (`?` prefix) with a "Search in Town Square" hint |
| 2 | Type a word from a message you sent earlier | Query appears after the `?` |
| 3 | Press `Enter` | Results appear listing matching posts |
| 4 | Navigate results with `↑`/`↓` | Results are highlighted |
| 5 | Press `Esc` | The palette closes |

**What this validates:** Search posts
(`POST /api/v1/channels/{channelId}/posts/search`), the palette's search mode.

:::tip[Tag Search]
You can include `#tag` in your search query to filter results by tags. For
example, `deploy #release` searches for posts containing "deploy" that are also
tagged with `release`. See Scenario 17 for a full walkthrough.
:::

#### Scenario 8: Slash Commands in the Palette

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Press `Ctrl+K` | The palette opens listing channels |
| 2 | Type `/` | The palette switches to command mode listing slash commands |
| 3 | Type a few characters to filter | List narrows to matching commands |
| 4 | Press `Esc` | The palette closes |

**What this validates:** Command retrieval (`GET /api/v1/commands`), fuzzy
filtering, the palette's `/` command mode.

#### Scenario 9: Unread Badge

This scenario requires two terminals.

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | In terminal A (Alice), switch to another channel or DM via `Ctrl+K` | Alice is no longer viewing Town Square |
| 2 | In terminal B (Bob), send a message in Town Square | Bob's message is created |
| 3 | In terminal A (Alice), press `Ctrl+K` | **Town Square** shows an unread badge `(1)` and sorts to the top |
| 4 | In terminal A, select Town Square | The badge clears on the next palette open |

**What this validates:** Channel member tracking
(`GET /api/v1/channels/{channelId}/members`), unread count computation,
view channel (`POST /api/v1/channels/{channelId}/members/me/view`).

#### Scenario 10: Theme Switching

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Quit the TUI (`Ctrl+C`) | TUI exits |
| 2 | Relaunch with `CHIT_THEME=catppuccin` (see command below) | TUI opens with the Catppuccin color scheme |
| 3 | Visually verify colors differ from Tokyo Night | Background, accent, and text colors are different |

```bash
CHIT_THEME=catppuccin \
CHIT_SERVER_URL=http://localhost:4455 \
./bin/chit-tui
```

**What this validates:** Theme loading via `CHIT_THEME` environment variable.

#### Scenario 11: Create a Team Channel

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Press `Ctrl+N` | Channel creator overlay opens |
| 2 | Type `Deploy` in the Display Name field | Name field auto-fills with `deploy` |
| 3 | Press `Tab` to skip to the Purpose field | Cursor moves to the purpose textarea |
| 4 | Type `Deployment coordination` | Purpose text appears |
| 5 | Press `Enter` | Channel is created; the action bar shows **UAT Team > Deploy** |
| 6 | Press `Ctrl+K` | **Deploy** is listed in the palette |

**What this validates:** Channel creation (`POST /api/v1/channels`), auto-slug
generation, channel creator overlay UI, palette refresh.

#### Scenario 12: Create a Direct Message

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Press `Ctrl+D` | The palette opens in people mode (`@` prefix) |
| 2 | Type `bob` and pause briefly | Search results show **@bob (Bob Baker)** |
| 3 | Press `Enter` | DM channel is created; the history pane switches to the 1:1 conversation and the action bar shows **bob** |
| 4 | Send a message: `Hey Bob!` | Message appears in the DM history |

**What this validates:** User search (`POST /api/v1/users/search`), DM channel
creation (`POST /api/v1/channels/direct`), the palette's `@` people mode.

#### Scenario 13: Create a Private Channel with Members

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Press `Ctrl+N` | Channel creator overlay opens |
| 2 | Type `War Room`, then use `←`/`→` on the type row to select **Private** | Type toggle shows `[Private]` |
| 3 | Press `Enter` | The member picker opens |
| 4 | Type `bob`, press `Enter` to search, then `Tab` on **Bob Baker** | Bob is selected (chip appears) |
| 5 | Press `Enter` | The private channel is created with Bob added; the action bar shows **UAT Team > War Room** |
| 6 | Repeat steps 1–3, then press `Esc` at the member picker | The channel is still created — Esc only skips member selection |

**What this validates:** Private channel creation, the member picker overlay
(multi-select with `Tab`), member addition
(`POST /api/v1/channels/{channelId}/members`), and Esc-skips-members behavior.

#### Scenario 14: Tag a Post

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Focus the history pane (`Tab`) | A post is highlighted with cursor |
| 2 | Press `t` | Tag picker overlay opens with a filter input |
| 3 | Type `urgent` in the filter | Tag list filters (may be empty if no tags exist yet) |
| 4 | Press `Ctrl+N` | A new `urgent` tag is created and applied to the post |
| 5 | Press `Esc` | Tag picker closes |
| 6 | Observe the post in the viewport | The post shows a `#urgent` badge |

**What this validates:** Tag creation (`POST /api/v1/tags`), tag application
(`POST /api/v1/posts/{postId}/tags`), tag badge rendering, tag picker overlay UI.

#### Scenario 15: Toggle Tags On/Off

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to the post tagged in Scenario 14 | Post shows `#urgent` badge |
| 2 | Press `t` | Tag picker opens; `urgent` shows `[x]` (checked) |
| 3 | Press `Enter` on `urgent` | Tag is removed; checkbox changes to `[ ]` |
| 4 | Press `Enter` on `urgent` again | Tag is re-applied; checkbox changes to `[x]` |
| 5 | Press `Esc` | Tag picker closes |

**What this validates:** Tag toggle on/off, `AddTagToPost` / `RemoveTagFromPost`
API calls, tag picker state updates.

#### Scenario 16: Hashtag Syntax in Messages

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Focus the input area | Cursor appears in the input box |
| 2 | Type `Deploy build 42 is ready #release #deploy` and press `Enter` | Message is sent |
| 3 | Observe the post in the viewport | Content shows `Deploy build 42 is ready` (hashtags stripped) |
| 4 | Observe the post badges | The post shows `#release` and `#deploy` tag badges |

**What this validates:** Hashtag parsing (`StripHashtags`), automatic tag
creation and application on post creation, tag badges rendered from post tags.

#### Scenario 17: Search by Tag

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Press `Ctrl+S` | The palette opens in search mode |
| 2 | Type `#urgent` and press `Enter` | Search results show posts tagged with `urgent` |
| 3 | Clear and type `build #release` and press `Enter` | Results show posts matching "build" that also have the `release` tag |
| 4 | Press `Esc` | The palette closes |

**What this validates:** Tag-based search via `#tag` syntax in search mode,
hybrid text + tag search, tag name to ID resolution.

#### Scenario 18: Runtime Theme Switching with Tags

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Focus the input and type `/skin`, press `Enter` | Skin picker overlay opens |
| 2 | Navigate to **Catppuccin** and press `Enter` | Theme changes; all colors update |
| 3 | Observe a tagged post | Tag badges use the Catppuccin `#a6e3a1` green color |
| 4 | Press `t` on a post | Tag picker overlay uses Catppuccin theme styles |

**What this validates:** Tag badge and tag picker styles update correctly when
the theme is hot-swapped via `/skin`.

#### Scenario 19: Session Persistence

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Quit the TUI (`Ctrl+C`) after a successful login | TUI exits |
| 2 | Relaunch with `CHIT_SERVER_URL=http://localhost:4455 ./bin/chit-tui` | TUI skips login; goes directly to the main view |
| 3 | Press `Ctrl+K` | Channels load without re-entering credentials |

**What this validates:** Session token persistence at
`~/.config/chit-tui/session.json`, stored token validation via Kratos
`CheckSession` on startup.

#### Scenario 20: Logout

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Focus the input and type `/logout`, press `Enter` | Login screen reappears |
| 2 | Enter a different user's email and password (e.g., `bob@example.com`) | Login succeeds; the action bar and palette show Bob's teams and channels |
| 3 | Quit and relaunch | TUI auto-loads Bob's session (no login screen) |

**What this validates:** `/logout` slash command, session clearing,
re-authentication to a different account, new session persistence.

#### Scenario 21: Login Error Handling

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Launch with a fresh session file: `CHIT_SESSION_FILE=/tmp/test-login.json` | — |
| 2 | Launch chit-tui | Login screen appears |
| 3 | Enter `alice@example.com` with password `wrong` and press `Enter` | Error message displayed on login screen |
| 4 | Enter the correct password `Password1!` and press `Enter` | Login succeeds; main view loads |

**What this validates:** Login error display, recovery from failed login
attempt, Kratos error handling.

#### Scenario 22: Mouse, Action Bar, and Help

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Click **[^K Jump]** on the action bar | The palette opens |
| 2 | Click a channel row inside the palette | The palette closes and the channel loads |
| 3 | Scroll the mouse wheel over the history pane | The history scrolls (when it overflows the pane) |
| 4 | Click **[? Help]** on the action bar | The help overlay opens listing keybindings and mouse actions |
| 5 | Press any key (or click anywhere) | The help overlay closes |
| 6 | Focus the history pane and press `?` | The help overlay opens again |

**What this validates:** Mouse support (action-bar hit-testing, palette row
clicks, wheel scroll), the help overlay, keyboard/mouse parity.

### Step 4: Tear Down the UAT Server

When you are done testing, stop the UAT environment from the Chit server repo:

```bash
cd ../chit
make uat-down
```

---

## Part 2: Automated E2E Tests

Chit TUI also ships with an automated end-to-end test suite that validates
the API and WebSocket clients programmatically. The tests run inside a Podman
pod — no manual interaction required.

### Environment Setup

The `make test-e2e` target automates the entire pipeline. Under the hood it:

1. **Builds the e2e test image** (`chit-tui-e2e:latest`) from `Containerfiles/Containerfile.e2e`
2. **Builds the Chit server image** (`chit-e2e:latest`) from the server repo's `Containerfiles/Containerfile.e2e`
3. **Creates a Podman pod** (`chit-tui-e2e`) with a shared network namespace
4. **Starts PostgreSQL** — `postgres:17-alpine` with database `chit`
5. **Starts the Chit server** — waits for Postgres, runs migrations, seeds UAT data, starts `chitd`
6. **Runs the e2e tests** — waits for `chitd` to respond to `/api/v1/system/ping`, then executes all tests
7. **Tears down the pod** regardless of pass or fail

### Quick Start

```bash
# Run everything (build + test + cleanup)
make test-e2e
```

### Step-by-Step (Manual)

If you prefer to run each phase individually:

```bash
# 1. Build the test runner image
make e2e-build

# 2. Build the Chit server image
make e2e-build-server

# 3. Run the full test pipeline
make test-e2e

# 4. Clean up a stuck pod (if something went wrong)
make test-e2e-clean
```

### Acceptance Tests

All tests are in `tests/e2e/` and guarded by the `//go:build e2e` build tag,
so they are excluded from `make test` and only run during `make test-e2e`.

#### API Tests

These tests validate the HTTP API client against the real Chit server.

##### TestGetMe

Authenticates as Alice and fetches her own profile. Verifies that the returned
username is `"alice"` and email is `"alice@example.com"`, confirming that
authentication and user retrieval work end-to-end.

**Endpoint:** `GET /api/v1/users/me`

##### TestGetMyTeams

Fetches Alice's team memberships and asserts that at least one team is returned
and that `uat-team` is present in the list.

**Endpoint:** `GET /api/v1/users/me/teams`

##### TestGetMyChannels

Looks up Alice's teams, finds `uat-team`, then fetches her channels for that
team. Verifies that `town-square` appears in the channel list.

**Endpoints:** `GET /api/v1/users/me/teams`, `GET /api/v1/users/me/teams/{teamId}/channels`

##### TestCreateAndGetPost

Creates a new post in `town-square` as Alice with a unique timestamp-based
message, then retrieves it by ID. Confirms the returned content matches what
was sent.

**Endpoints:** `POST /api/v1/posts`, `GET /api/v1/posts/{postId}`

##### TestGetChannelPosts

Creates a post in `town-square`, then fetches the channel's post list. Verifies
that at least one post is returned in the paginated result.

**Endpoints:** `POST /api/v1/posts`, `GET /api/v1/channels/{channelId}/posts`

##### TestPinUnpinPost

Creates a post, pins it, retrieves it to verify `is_pinned` is `true`, unpins
it, and retrieves it again to verify `is_pinned` is `false`. Tests the full
pin/unpin lifecycle.

**Endpoints:** `POST /api/v1/posts`, `POST /api/v1/posts/{postId}/pin`, `POST /api/v1/posts/{postId}/unpin`, `GET /api/v1/posts/{postId}`

##### TestGetThread

Creates a root post in `town-square`, then creates a reply to it (setting
`root_id`). Fetches the thread and verifies that the reply count is at least 1
and the post list contains at least 2 entries (root + reply).

**Endpoints:** `POST /api/v1/posts`, `GET /api/v1/posts/{postId}/thread`

##### TestSearchPosts

Creates a post with a unique random term, waits briefly for indexing, then
searches for that term within the channel. Verifies that at least one matching
result is returned.

**Endpoints:** `POST /api/v1/posts`, `POST /api/v1/channels/{channelId}/posts/search`

##### TestGetUsersByIDs

Fetches the user IDs for both Alice and Bob via `GetMe`, then performs a batch
lookup by passing both IDs. Verifies that exactly 2 users are returned.

**Endpoints:** `GET /api/v1/users/me`, `POST /api/v1/users/ids`

##### TestViewChannel

Marks `town-square` as viewed by Alice. Verifies that the endpoint returns
successfully with no error. This is the API call the TUI uses to clear unread
badges.

**Endpoint:** `POST /api/v1/channels/{channelId}/members/me/view`

##### TestCreateDMAndPost

Creates a direct message channel between Alice and Bob, verifies that the
returned channel has type `"D"` (Direct), then posts a message into it.
Confirms the post's `channel_id` matches the DM channel.

**Endpoints:** `GET /api/v1/users/me`, `POST /api/v1/channels/direct`, `POST /api/v1/posts`

#### WebSocket Tests

These tests validate the real-time WebSocket event stream.

##### TestWSConnectAndReceiveEvent

Opens a WebSocket connection as Alice, then posts a message as Bob in
`town-square` via the HTTP API. Listens on Alice's WebSocket for up to 10
seconds and asserts that a `posted` event is received, confirming that the
server broadcasts real-time events to connected channel members.

**WebSocket:** `ws://localhost:8065/api/v1/websocket`
**HTTP Endpoint:** `POST /api/v1/posts`

## Troubleshooting

### Pod cleanup

If a previous run was interrupted, a stale pod may block the next run:

```bash
make test-e2e-clean
```

### Viewing container logs

While the pod is running (or after a failure before cleanup), inspect logs:

```bash
podman logs chit-tui-e2e-pg      # PostgreSQL
podman logs chit-tui-e2e-chitd   # Chit server
podman logs chit-tui-e2e-tests   # Test runner
```

### Server not ready

The test runner waits up to 60 seconds for `chitd` to respond to
`/api/v1/system/ping`. If the server takes longer (first-time image build,
slow CI runner), increase the timeout:

```bash
podman run --rm --pod chit-tui-e2e \
  -e MAX_WAIT=120 \
  -e CHIT_E2E_SERVER_URL=http://localhost:8065 \
  chit-tui-e2e:latest
```

### Running a single test locally

If you have a running UAT environment (via the Chit server's `make uat-up`),
you can run individual tests without the full pod pipeline:

```bash
go test -tags e2e -v -run TestGetMe ./tests/e2e/...
```
