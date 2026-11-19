---
id: "01a1f5dd-dc78-70a8-b472-d53b133f081f"
title: "ADR-0012: Attachments in SeaweedFS, checked by imagecheck"
type: "ADR"
tags:
  - "adr"
  - "attachments"
  - "security"
created: "2026-11-19T16:12:00-06:00"
timestamp: "2026-11-19T16:12:00-06:00"
description: "chitd keeps the bytes of image attachments in an S3 bucket (SeaweedFS), checks and encodes each upload again with pkg/imagecheck, and serves it only to channel members."
---

# ADR-0012: Attachments in SeaweedFS, checked by imagecheck

**Status:** Accepted
**Date:** 2026-11-19 (records the decision of PR #7)
**Source:** HALO prerequisite P-CHIT-5. `CHANGELOG.md`. [ISSUE-014](/issues/issue-014-merge-main-and-bring-its-new-code-under-the-gates--01a1f5dc77bc.md) merged it into the wiki.

## Context
HALO shows images in a Conversation. An uploaded image can carry a script, a second file type or metadata with personal data.

## Decision
- `POST /channels/{id}/files` uploads an image. `GET /files/{id}` serves its bytes, and `GET /files/{id}/info` its metadata. A post attaches up to 4 files with `file_ids`.
- `pkg/imagecheck` accepts PNG, JPEG, WebP and GIF only. It refuses SVG, a mismatch of the declared and the found type, and a polyglot. It encodes each image again in pure Go, which removes EXIF.
- `internal/blob` keeps the bytes in the bucket `chit-files` under `<channel_id>/<file_id>`. The same bytes share one object in a channel, but not across channels.
- Only the uploader reads a file before a post attaches it. Then each member can read it. Each file response has `nosniff`, `sandbox` and `Content-Disposition`.
- A sweep deletes uploads that no post attached and the objects of deleted files.
- Attachments are off until the operator sets `CHIT_S3_ENDPOINT`.

## Consequences
- `pkg/imagecheck` has no chit dependencies. It has its own `internal/assert`, so HALO can use it alone.
- The on-behalf-of allowlist has the upload and download routes ([On-behalf-of requests](/specs/on-behalf-of-requests--01a1f5379dea.md)).
