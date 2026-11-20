---
id: "01a1f771-12ef-7734-9f5c-84d28ee90e9d"
title: "ISSUE-024: The TUI e2e suite and the live run do not cover the HALO features"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-18"
  - "chit-tui"
  - "open"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-19T23:32:25-06:00"
description: "The TUI e2e suite tests none of the HALO routes and events, and no live run checks the TUI against the full stack."
---

# ISSUE-024: The TUI e2e suite and the live run do not cover the HALO features

**Status/Phase:** Open · Phase 18

## Description
The TUI e2e suite tests only the routes and events from before the HALO work. The verify skill names an old PostgreSQL image.

## Acceptance Criteria
- [ ] The TUI e2e suite tests the tag, channel-tag, file, stream_status, 429 and 409 paths.
- [ ] A live run through Oathkeeper shows each new feature.
- [ ] The verify skill, the UAT tutorial and the changelogs are current.
- [ ] make check and the full-stack e2e script pass.
