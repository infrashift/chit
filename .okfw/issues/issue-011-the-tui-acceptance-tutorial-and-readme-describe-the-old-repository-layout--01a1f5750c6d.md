---
id: "01a1f575-0c6d-725b-91a4-da6dddd183d9"
title: "ISSUE-011: The TUI acceptance tutorial and README describe the old repository layout"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "docs"
  - "tui"
created: "2026-11-19T14:17:31-06:00"
timestamp: "2026-11-19T14:17:31-06:00"
description: "The TUI acceptance tutorial describes a separate chit-tui repository, and the README file tree omits newer files."
---

# ISSUE-011: The TUI acceptance tutorial and README describe the old repository layout

**Status/Phase:** Open · Docs debt

## Description
The agents of Phase 10 found facts in the TUI documentation that do not match the monorepo. They kept the facts and changed only the words.

- `docs/src/content/docs/tui/tutorials/user-acceptance-testing.md` describes a separate `chit-tui` repository next to the server repository. Its code blocks use `cd ../chit`. In the monorepo, `CHIT_SRC` has the default `../..`.
- The file tree in `clients/chit-tui/README.md` does not show `threads.go`, `clipboard.go`, `keymap.go`, `internal/assert` and `internal/errlog`.

## Acceptance Criteria
- [ ] The tutorial uses the monorepo paths and the `CHIT_SRC` default.
- [ ] The README file tree agrees with `clients/chit-tui/`.
- [ ] `make docs-lexis` reports zero findings.

## Related
- [ISSUE-010](/issues/issue-010-asd-ste100-documentation--01a1f5379e74.md): the STE rewrite that found these facts
