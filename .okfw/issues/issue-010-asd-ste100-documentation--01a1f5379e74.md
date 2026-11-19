---
id: "01a1f537-9e74-7dfc-9d5a-65479eeae041"
title: "ISSUE-010: ASD-STE100 documentation"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-10"
  - "docs"
  - "ste"
  - "closed"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T14:38:12-06:00"
description: "The Markdown documentation and the wiki give zero lexis findings."
---

# ISSUE-010: ASD-STE100 documentation

**Status/Phase:** Closed · Phase 10

## Description
The Markdown documentation and the wiki give zero lexis findings.

## Acceptance Criteria
- [x] `make docs-lexis` reports zero findings.
- [x] Each `vocabulary.cue` term has a reason.
- [x] `make check` passes.

## Resolution
Done on 2026-11-19 in Phase 10 ([ADR-0005](/decisions/adr-0005-lexis-checks-the-documentation-with-a-repository-vocabulary--01a1f5379e0c.md)).

- `make docs-lexis` checks 69 files and reports zero findings. Before, the files had 1,175 findings.
- `DOCS_LEXIS_PENDING` is gone. All the documentation is in `DOCS_LEXIS_PATHS`.
- `vocabulary.cue` has 15 new terms for security, server features, the TUI and the tests. Each term has a reason.
- The rewrite also corrected facts from the code. Examples: the 19 WebSocket event types, the store decorators that do not exist yet, and the TUI API names.
- [ISSUE-011](/issues/issue-011-the-tui-acceptance-tutorial-and-readme-describe-the-old-repository-layout--01a1f5750c6d.md) records the old repository layout in the TUI tutorial.
- The docs site builds: 30 pages.
- `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
