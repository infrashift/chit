---
id: "01a1f982-f74b-744c-98a7-ed73e26b70ec"
title: "ADR-0018: Attachments in a text-only client"
type: "ADR"
tags:
  - "adr"
  - "chit-tui"
  - "attachments"
created: "2026-11-20T09:11:12-06:00"
timestamp: "2026-11-20T09:11:12-06:00"
description: "chit-tui shows each attachment as one line of text, uploads with /attach and alt text into a tray of up to four files, and saves files to download_dir without overwriting."
---

# ADR-0018: Attachments in a text-only client

**Status:** Accepted
**Date:** 2026-11-20
**Source:** [ISSUE-023](/issues/issue-023-the-tui-cannot-attach-or-save-files--01a1f77112e5.md). The server side is [ADR-0012](/decisions/adr-0012-attachments-in-seaweedfs-checked-by-imagecheck--01a1f5dddc78.md).

## Context
The server accepts up to four images on a post. chit-tui is text only: it cannot draw an image. It could not upload a file or save one.

## Decision
- **Show.** Each file of a post has one line: the name, the type, the size in pixels and in bytes, and the alt text. The post details (`i`) list the files too.
- **Upload.** `/attach <path>` or the file picker (`ui/filepick`) picks the file. Before any request, the client examines the extension (PNG, JPEG, WebP, GIF) and the size (10 MB). `ui/prompt` asks for the alt text, which the server keeps. `ui/activity` shows the upload until the server answers.
- **Tray.** The uploaded files wait in a tray of up to four files (`ui/chips`) on the upper border of the input. The next post, or the next reply, takes them. A refused post keeps the tray. `Backspace` in the empty input, `/detach` and a channel switch empty it. The server deletes an upload that no post takes after 24 hours.
- **Files alone.** While the tray holds files, `Enter` in an empty input sends a post with no text.
- **Save.** `s` in the post details downloads each file to `download_dir`. The client opens each new file with `O_CREATE|O_EXCL` and mode 0600, so it never writes over a file. It uses only the base name of the file, and a taken name gets a number, up to 100 tries. A failed download removes the part that it wrote. Each read stops at 10 MB.

## Consequences
- The record of an upload goes into the file cache at once, and the panes get it. Before this, the post that took the file showed a placeholder. The cache held the record, so the client did not fetch it. The panes never got it. The TUI e2e test found this.
- The client does not show the pixels of an image. A user who must see one saves it and opens it in another program.

## Related
- [ADR-0012](/decisions/adr-0012-attachments-in-seaweedfs-checked-by-imagecheck--01a1f5dddc78.md)
- [ADR-0015](/decisions/adr-0015-reusable-bubbles-in-internal-tui-ui--01a1f782e7a8.md)
