---
id: "01a1f575-0c6d-725b-91a4-da6dddd183d9"
title: "ISSUE-011: The TUI acceptance tutorial and README describe the old repository layout"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "docs"
  - "tui"
  - "closed"
created: "2026-11-19T14:17:31-06:00"
timestamp: "2026-11-19T15:22:40-06:00"
description: "The TUI acceptance tutorial describes a separate chit-tui repository, and the README file tree omits newer files."
---

# ISSUE-011: The TUI acceptance tutorial and README describe the old repository layout

**Status/Phase:** Closed · Docs debt

## Description
The agents of Phase 10 found facts in the TUI documentation that do not match the monorepo. They kept the facts and changed only the words.

- `docs/src/content/docs/tui/tutorials/user-acceptance-testing.md` describes a separate `chit-tui` repository next to the server repository. Its code blocks use `cd ../chit`. In the monorepo, `CHIT_SRC` has the default `../..`.
- The file tree in `clients/chit-tui/README.md` does not show `threads.go`, `clipboard.go`, `keymap.go`, `internal/assert` and `internal/errlog`.

## Acceptance Criteria
- [x] The tutorial uses the monorepo paths and the `CHIT_SRC` default.
- [x] The README file tree agrees with `clients/chit-tui/`.
- [x] `make docs-lexis` reports zero findings.

## Resolution
Done on 2026-11-19.

- The tutorial shows the monorepo: chit-tui is in `clients/chit-tui`, and the TUI Makefile finds the server source at `../..` (`CHIT_SRC`).
- The UAT steps go to the root of the repository with `cd ../..`, not to a sibling `../chit`.
- The tutorial names the services of `deploy/chit-uat.kube.yml`: Kratos, Oathkeeper, Keto and ZincSearch. It named Vault before, which the UAT pods do not run.
- The README file tree shows `assert/`, `errlog/`, `doc.go`, `keymap.go`, `threads.go` and `clipboard.go`, and `update.go` with its routers.
- `make docs-lexis` reports zero findings.

## Related
- [ISSUE-010](/issues/issue-010-asd-ste100-documentation--01a1f5379e74.md): the STE rewrite that found these facts
