---
id: "01a1f585-7d5f-7145-8f27-8097b59917f0"
title: "ISSUE-012: ValidateTagName cuts the reserved prefix at the wrong byte"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "bug"
  - "tags"
  - "closed"
created: "2026-11-19T14:35:28-06:00"
timestamp: "2026-11-19T15:17:20-06:00"
description: "A profile/ tag name that spells the prefix with a character whose lowercase is shorter, for example İ, keeps the slash after the cut, and chitd refuses it."
---

# ISSUE-012: ValidateTagName cuts the reserved prefix at the wrong byte

**Status/Phase:** Closed · Bug

## Description
`internal/model/tag.go` `ValidateTagName` finds the reserved prefix `profile/` in the key, which is lowercase. Then it cuts the prefix from the name with `name[len(ReservedTagPrefix):]`. The comment says that the prefix has the same byte length in the name and in the key. That is false for `İ` (U+0130, two bytes), which becomes `i` (one byte). For `profİle/x`, the cut gives `/x`, and chitd refuses the name with 400.

The R5 agent of Phase 9 found this. The assertion `len(name) >= len(ReservedTagPrefix)` holds, so the bug gives a wrong refusal, not a panic.

## Acceptance Criteria
- [x] `ValidateTagName` cuts the prefix by runes of the name, not by bytes of the key.
- [x] A test covers `profİle/x` and the comment is correct.

## Resolution
Done on 2026-11-19.

- `ValidateTagName` lowercases the name itself and drops the prefix by runes with the new `dropRunes`. Rune i of the key is rune i of the name, because lowercase maps each rune to one rune.
- `IsReservedTagName` does not change.
- New test cases: `profİle/coder` and `PROFİLE/İstanbul` are valid. chitd refuses `profİle/` and `profİle/a/b`, the same as without `İ`.
- The CHANGELOG records the fix.
- `make check` and `make test-integration` pass.

## Related
- [ISSUE-009](/issues/issue-009-r5-assertion-density--01a1f5379e6c.md)
