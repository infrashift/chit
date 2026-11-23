---
x-okfw-id: "01a1f771-12d1-7d27-8f8d-a1921bf7db81"
title: "ISSUE-021: The TUI does not show agents, provenance, attachments or stream status"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-15"
  - "chit-tui"
  - "closed"
x-okfw-created: "2026-11-19T23:32:25-06:00"
x-okfw-timestamp: "2026-11-20T08:42:27-06:00"
description: "A post that an agent makes for a person looks like the person typed it, attachments are invisible, and the TUI ignores stream_status."
---

# ISSUE-021: The TUI does not show agents, provenance, attachments or stream status

**Status/Phase:** Closed · Phase 15

## Description
The TUI shows agents and bots as people. It does not show the on-behalf-of actor or the provenance of a post. A post with only attachments has an empty body. The TUI ignores the stream_status event, so the user does not see that an agent writes a reply.

## Acceptance Criteria
- [x] The post header shows the agent or bot badge, the via actor and the provenance badge.
- [x] Each attachment has a line under the post.
- [x] The TUI gets file information in batches that obey the rate limit.
- [x] stream_status shows an indicator above the input.
- [x] A detail overlay shows the provenance and the files of a post.
- [x] make check passes.

## Resolution
Done on 2026-11-20.

- The post header shows `[agent]` or `[bot]`, `[via @client]` and `[provenance]`. The palette, the member picker and the mention popup mark agents and bots.
- Each file of a post has a line. A post with files and no text has no empty line. `post.Directory` holds the names, the actor types and the file records, and it tells which cached posts to draw again.
- The client gets file records 40 at a time, one batch after the other. A 429 stops the batch, and the rest waits for `Retry-After`. The queue and the cache have caps.
- `stream_status` shows `… is responding` on the lower border of the main pane (`ui/activity`). A post of the sender, `finished`, a channel switch or 30 seconds clear it.
- `i` opens the post details in a new Bubble, `ui/infobox`.
- The authors of thread replies now load. Before, they showed as an ID.
- A new e2e test runs the real TUI model against chitd. It shows the badges, a file line and a stream indicator of real HALO data. 29 of 29 TUI e2e tests pass.
