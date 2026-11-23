---
x-okfw-id: "01a1f972-487b-7e21-b046-617e191908bc"
title: "ADR-0017: Tag management in the TUI"
type: "ADR"
tags:
  - "adr"
  - "chit-tui"
  - "tags"
x-okfw-created: "2026-11-20T08:52:58-06:00"
x-okfw-timestamp: "2026-11-20T08:52:58-06:00"
description: "chit-tui renames and deletes a tag only for its owner, applies the server's tag events at once, shows the channel tags on the pane border and finds channels by tag with /chtag."
---

# ADR-0017: Tag management in the TUI

**Status:** Accepted
**Date:** 2026-11-20
**Source:** [ISSUE-022](/issues/issue-022-the-tui-does-not-manage-tags-as-the-halo-server-permits--01a1f77112db.md).

## Context
The server gives each tag an owner and a scope, and it sends `tag_updated`, `tag_deleted` and `channel_tags_updated`. The server sends no `post_tags_updated` after a delete. The TUI ignored these events, so a renamed or deleted tag stayed until a restart. The TUI could not rename or delete a tag, and it had no channel tags.

## Decision
- **Keys in the picker.** The filter of the tag picker takes each printable key, so the picker uses `Ctrl+R` to rename and `Ctrl+X` to delete. The picker sends a request. The root model checks that the user owns the tag before it asks the server. The server checks it again.
- **Bubbles.** The rename prompt is `ui/prompt`. It is the first overlay in the table, so it gets the keys first, and the View draws it last, on top of the picker. The delete asks with `ui/confirm` and the key `Ctrl+X`. A 409 shows in the prompt, and the prompt stays open.
- **Events.** One function, `applyTagEvent`, applies each tag event. A rename replaces the tag in the known tags, on the posts, in the picker and on the channel. A delete removes it from each of them.
- **Channel tags.** The tags of the open channel show as `ui/chips` on the upper border of the main pane. A channel switch loads them, and `GET /tags?channel_id=` adds the channel tags that other members made. `/chtag add` makes a channel tag when the name has none. `/chtag rm` asks first, with `y`. `/chtag find` lists up to 9 channels in `ui/infobox`, and a digit key opens one.

## Consequences
- The View draws the overlays in the reverse order of the table. The table order is also the order of the keys, so the overlay that gets the keys is on top.
- The client sends two more requests when it opens a channel.

## Related
- [ADR-0015](/decisions/adr-0015-reusable-bubbles-in-internal-tui-ui--01a1f782e7a8.md)
