---
id: "01a1f771-12db-7a0d-b70b-629c9e731a39"
title: "ISSUE-022: The TUI does not manage tags as the HALO server permits"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-16"
  - "chit-tui"
  - "open"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-19T23:32:25-06:00"
description: "The TUI ignores tag_updated, tag_deleted and channel_tags_updated, and it cannot rename or delete a tag or use channel tags."
---

# ISSUE-022: The TUI does not manage tags as the HALO server permits

**Status/Phase:** Open · Phase 16

## Description
A renamed or deleted tag stays in the TUI until it starts again. The TUI cannot rename or delete a tag. It has no channel tags.

## Acceptance Criteria
- [ ] tag_updated, tag_deleted and channel_tags_updated update the TUI.
- [ ] The owner can rename and delete a tag from the tag picker.
- [ ] The channel header shows the channel tags. /chtag adds, removes and finds channel tags.
- [ ] make check passes.
