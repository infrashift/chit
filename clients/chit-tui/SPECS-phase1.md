# SPECS-phase1.md: Technical Specification

## 1. State Management & Architecture

* **MVU Pattern:** Implement a root model that delegates `Update` and `View` calls to sub-components: `Sidebar`, `Viewport`, `Input`, and `ThreadPanel`.
* **Async Commands:** Use `tea.Cmd` for all I/O. The WebSocket listener must run in a background goroutine, piping `model.WebSocketEvent` into the main update loop.
* **Resource Mapping:** All internal IDs must use the UUIDv7 format as defined in the backend.

## 2. UI Component Specifications

* **Sidebar (list bubble):**
* Fetch teams via `GET /users/me/teams`.
* Fetch channels via `GET /users/me/teams/{id}/channels`.
* Highlight channels with unread counts based on `msg_count` vs `last_viewed_at`.


* **Messenger (viewport & textarea bubbles):**
* Render posts using `glamour` for Markdown.
* Trigger `POST /channels/{id}/members/me/view` when a channel is focused to clear unread states.
* Display "Encrypted" badges for messages where `content_encrypted` was used.


* **Threads:**
* Toggle a side-panel for threads using `GET /posts/{id}/thread`.
* Implement "Auto-follow" on reply as per server logic.



## 3. Custom Bubbles to Implement

* **`PostBubble`:** A standalone component for a single message.
* **Logic:** Handles hover/focus states for pinning (`POST /posts/{id}/pin`) or tagging.


* **`CmdPalette`:** A fuzzy-finder for slash commands.
* **Data Source:** `GET /commands`.
* **Action:** Dispatches the selected command slug to the input buffer.



## 4. Security & Networking

* **Oathkeeper Proxy:** All HTTP/WS clients must inject the `X-Session-Token` (retrieved from local Kratos config) into every request.
* **Concurrency:** Implement a `Sequence` counter for WebSocket events to prevent UI jitter during high-volume bursts.