---
x-okfw-id: "01a1f771-12e5-7781-8e2b-50aa21a079c2"
title: "ISSUE-023: The TUI cannot attach or save files"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-17"
  - "chit-tui"
  - "closed"
x-okfw-created: "2026-11-19T23:32:25-06:00"
x-okfw-timestamp: "2026-11-20T09:14:04-06:00"
description: "The TUI cannot upload an image to a post or save the attachment of a post to disk."
---

# ISSUE-023: The TUI cannot attach or save files

**Status/Phase:** Closed · Phase 17

## Description
The server accepts image attachments. The TUI has no way to upload a file, to send a post with files, or to save a file to disk.

## Acceptance Criteria
- [x] /attach uploads an image, with a file picker when the command has no path.
- [x] A tray shows up to four files that wait for the next post. A post sends them.
- [x] The detail overlay saves the files of a post to download_dir, with no overwrite.
- [x] make check passes.

## Resolution
Done on 2026-11-20.

- `/attach <path>`, or `/attach` and the file picker (`ui/filepick`), uploads an image after the alt text (`ui/prompt`). The client examines the type and the size first. `ui/activity` shows the upload.
- A tray of up to four files (`ui/chips`) waits on the input border. The next post or reply takes the files, and `Enter` in an empty input sends them alone. `Backspace`, `/detach` and a channel switch empty the tray. A refused post keeps it.
- `s` in the post details saves the files to `download_dir` (new setting, also `CHIT_DOWNLOAD_DIR`). The client never writes over a file and uses only the base name.
- The TUI e2e test found a bug: the post that took an upload showed a placeholder. The record of an upload now goes to the panes at once.
- A new e2e test attaches, sends and saves a file through the real TUI. 31 of 31 TUI e2e tests pass.
- [ADR-0018](/decisions/adr-0018-attachments-in-a-text-only-client--01a1f982f74b.md) records the decisions.
