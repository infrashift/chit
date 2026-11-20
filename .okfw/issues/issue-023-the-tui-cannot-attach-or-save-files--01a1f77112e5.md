---
id: "01a1f771-12e5-7781-8e2b-50aa21a079c2"
title: "ISSUE-023: The TUI cannot attach or save files"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-17"
  - "chit-tui"
  - "open"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-19T23:32:25-06:00"
description: "The TUI cannot upload an image to a post or save the attachment of a post to disk."
---

# ISSUE-023: The TUI cannot attach or save files

**Status/Phase:** Open · Phase 17

## Description
The server accepts image attachments. The TUI has no way to upload a file, to send a post with files, or to save a file to disk.

## Acceptance Criteria
- [ ] /attach uploads an image, with a file picker when the command has no path.
- [ ] A tray shows up to four files that wait for the next post. A post sends them.
- [ ] The detail overlay saves the files of a post to download_dir, with no overwrite.
- [ ] make check passes.
