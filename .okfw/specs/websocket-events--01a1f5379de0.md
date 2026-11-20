---
id: "01a1f537-9de0-7254-b2fa-32076fce034d"
title: "WebSocket events"
type: "Spec"
tags:
  - "spec"
  - "websocket"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "chitd sends 19 event types over /api/v1/websocket to the members of the channel or team of each event."
---

# WebSocket events

A client connects to `/api/v1/websocket`. chitd authenticates the connection like any other API request. The wire format is in `docs/src/content/docs/api-reference/websocket-protocol`.

## Event types
| Event | Meaning |
|---|---|
| `posted` | A user made a post or a reply. |
| `post_edited` | A user edited a post. |
| `post_deleted` | A user deleted a post. |
| `post_pinned`, `post_unpinned` | A user pinned or unpinned a post. |
| `typing` | A channel member types. |
| `channel_created`, `channel_updated`, `channel_deleted` | A channel, DM or group channel changed. |
| `user_added`, `user_removed` | A channel got or lost a member. |
| `thread_updated` | The counters of a thread changed. |
| `post_tags_updated` | The tags of a post changed. |
| `channel_tags_updated` | The tags of a channel changed. |
| `tag_updated`, `tag_deleted` | A user renamed or deleted a tag. The owner gets a private tag event, and the channel gets a channel tag event. |
| `stream_status` | A member, or an agent for a member, writes a reply. chitd sends it on and does not store it. |
| `mentioned` | A post names the connected user with `@`. |
| `command_response` | The reply to a slash command of the connected user. Only that user gets it. |

## Rules
- The hub sends a channel event to the channel members only. It sends a team event to the team members only.
- The hub registers a client before it starts the pumps.
- On the first connection of a user, the hub loads the channel and team IDs of the user. It holds channel and team events for that user until the load ends.
- A client message has a maximum of 4096 bytes. The hub closes a client with a full send buffer (256 events).
- Fan-out is in-process ([ADR-0007](/decisions/adr-0007-thin-pub-sub-envelopes-with-pg-listen-notify-by-default--01a1f5379e1a.md)).

## Related
- [chit architecture overview](/architecture/chit-architecture-overview--01a1f5379dd5.md)
