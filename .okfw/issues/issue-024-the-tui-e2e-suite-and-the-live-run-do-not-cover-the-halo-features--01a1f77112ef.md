---
id: "01a1f771-12ef-7734-9f5c-84d28ee90e9d"
title: "ISSUE-024: The TUI e2e suite and the live run do not cover the HALO features"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-18"
  - "chit-tui"
  - "closed"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-20T09:22:49-06:00"
description: "The TUI e2e suite tests none of the HALO routes and events, and no live run checks the TUI against the full stack."
---

# ISSUE-024: The TUI e2e suite and the live run do not cover the HALO features

**Status/Phase:** Closed · Phase 18

## Description
The TUI e2e suite tests only the routes and events from before the HALO work. The verify skill names an old PostgreSQL image.

## Acceptance Criteria
- [x] The TUI e2e suite tests the tag, channel-tag, file, stream_status, 429 and 409 paths.
- [x] A live run through Oathkeeper shows each new feature.
- [x] The verify skill, the UAT tutorial and the changelogs are current.
- [x] make check and the full-stack e2e script pass.

## Resolution
Done on 2026-11-20.

- Each phase added its own e2e tests. The TUI e2e suite has 31 tests: `halo_test.go` checks the HALO contract, and `tui_test.go` runs the real TUI model against chitd and SeaweedFS. The e2e pod runs SeaweedFS, a machine actor and a wider rate limit.
- `live_test.go` runs the TUI against the full stack. The TUI signs in on its own sign-in screen through Kratos and Oathkeeper, attaches a file, sees a tag rename arrive and changes the theme. It passes. In the e2e pod it skips, because `CHIT_LIVE_PROXY_URL` is not set.
- `tests/e2e/test_e2e.sh` on the full stack: 293 pass, 0 fail, 3 skips (scenario 2 on a used volume).
- The verify skill names the PostgreSQL 17 image, the automated checks and the HALO flows. The UAT tutorial has scenarios 23 to 28 and the new automated tests. The TUI overview, both changelogs and the TUI README are current.
