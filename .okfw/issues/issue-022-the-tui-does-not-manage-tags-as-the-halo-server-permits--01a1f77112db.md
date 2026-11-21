---
id: "01a1f771-12db-7a0d-b70b-629c9e731a39"
title: "ISSUE-022: The TUI does not manage tags as the HALO server permits"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-16"
  - "chit-tui"
  - "closed"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-20T08:55:39-06:00"
description: "The TUI ignores tag_updated, tag_deleted and channel_tags_updated, and it cannot rename or delete a tag or use channel tags."
---

# ISSUE-022: The TUI does not manage tags as the HALO server permits

**Status/Phase:** Closed · Phase 16

## Description
A renamed or deleted tag stays in the TUI until it starts again. The TUI cannot rename or delete a tag. It has no channel tags.

## Acceptance Criteria
- [x] tag_updated, tag_deleted and channel_tags_updated update the TUI.
- [x] The owner can rename and delete a tag from the tag picker.
- [x] The channel header shows the channel tags. /chtag adds, removes and finds channel tags.
- [x] make check passes.

## Resolution
Done on 2026-11-20.

- `applyTagEvent` applies `tag_updated`, `tag_deleted` and `channel_tags_updated` to the known tags, the posts, the tag picker and the channel.
- In the tag picker, `Ctrl+R` renames through `ui/prompt` and `Ctrl+X` deletes through `ui/confirm`. Both work only for the owner. A 409 shows in the prompt.
- The tags of the open channel show as `ui/chips` on the upper border of the main pane. `/chtag add`, `rm` and `find` change and find them. `find` lists the channels in `ui/infobox`, and a digit opens one.
- The View draws the overlays in the reverse order of the table, so the overlay that gets the keys is on top.
- A new e2e test runs bob's TUI against chitd while alice tags, renames and deletes. 30 of 30 TUI e2e tests pass.
- [ADR-0017](/decisions/adr-0017-tag-management-in-the-tui--01a1f972487b.md) records the decisions.
