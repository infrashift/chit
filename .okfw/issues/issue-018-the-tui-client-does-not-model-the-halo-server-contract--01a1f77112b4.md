---
x-okfw-id: "01a1f771-12b4-72b1-a286-0ae6b5588002"
title: "ISSUE-018: The TUI client does not model the HALO server contract"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-12"
  - "chit-tui"
  - "closed"
x-okfw-created: "2026-11-19T23:32:25-06:00"
x-okfw-timestamp: "2026-11-19T23:45:06-06:00"
description: "The TUI models, API client and errors do not know the attachments, provenance, actor types, tag routes, Retry-After or the refusal reason of the HALO server."
---

# ISSUE-018: The TUI client does not model the HALO server contract

**Status/Phase:** Closed · Phase 12

## Description
The server added attachments, on-behalf-of posts, provenance, actor types, scoped tags, channel tags, rate limits and the membership hook. The TUI models and the API client do not know these fields and routes. An error shows only the status code and the message. The TUI does not read the refusal reason or the Retry-After header. The local tag-name rules are not the same as the server rules.

## Acceptance Criteria
- [x] The models carry file_ids, actor_type, the tag timestamps, File, TaggedChannel, and the reason and detail of an error.
- [x] Post.OnBehalfOf and Post.Provenance read the post props.
- [x] APIError carries Reason, Detail and RetryAfter. UserMessage gives a short message for each stable error id.
- [x] The API client has the file, tag and channel-tag methods. The test doubles have them too.
- [x] ValidTagName obeys the server rules. The hashtag pattern accepts Unicode letters and digits.
- [x] make check passes.

## Resolution
Done on 2026-11-19.

- The models carry `file_ids`, `actor_type`, the tag timestamps, `File`, `TaggedChannel`, and the `reason` and `detail` of an error. `Post.OnBehalfOf` and `Post.Provenance` read the post props.
- `APIError` keeps `RetryAfter`, to a maximum of 60 seconds. `api.UserMessage` gives the text of the action bar.
- The client has `GetTags`, `RenameTag`, `DeleteTag`, the channel-tag methods, `FindChannelsByTags`, `UploadFile`, `GetFileInfo` and `DownloadFile`. Each reads at most 10 MB of a file and 64 KB of an error body.
- `model.ValidTagName` applies the rules of the server. A `#tag` accepts Unicode letters and digits.
- The TUI e2e pod runs SeaweedFS, a machine actor that may post on behalf of a person, and a wider rate limit. The new e2e tests cover each new route and the WebSocket events `tag_updated`, `tag_deleted`, `channel_tags_updated` and `stream_status`. 28 of 28 tests pass.
- The new code in `internal/api` and `internal/model` has 95% test coverage.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
