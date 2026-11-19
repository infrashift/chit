---
id: "01a1f537-9e3e-7986-9c81-1afde5cd6aa3"
title: "ISSUE-003: R7: check each return value"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-3"
  - "power-of-10"
  - "r7"
  - "closed"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:28:23-06:00"
description: "No discarded call result and no dropped error in either Go module."
---

# ISSUE-003: R7: check each return value

**Status/Phase:** Closed · Phase 3

## Description
No discarded call result and no dropped error in either Go module.

## Acceptance Criteria
- [x] semscope NASA-P10-07 with `go_types` reports zero findings.
- [x] `errcheck` has `check-blank` and `check-type-assertions`. `errorlint` is on.
- [x] The omission of NASA-P10-07 is gone from `.semscope.cue`.
- [x] `make check` and `make test-integration` pass.

## Resolution
Done on 2026-11-19 in Phase 3 ([ADR-0010](/decisions/adr-0010-errlog-checks-the-errors-that-a-caller-cannot-return--01a1f53e0eaa.md)).

- semscope NASA-P10-07 with `go_types` reports zero findings. Before, it reported 82.
- golangci-lint with `check-blank`, `check-type-assertions` and `errorlint` reports zero issues. Before, it reported 172 in the server module and 381 in the TUI module.
- The TUI `.golangci.yml` had the default caps on the issue count, so it showed only 53 issues. It now shows all of them, the same as the root file.
- Each module has `internal/errlog`. No suppression was necessary.
- Bug fix: the cleanup of the Keto integration test used `t.Context()`. Go cancels that context before the cleanup runs, so the deletes did not occur. The cleanup now uses `context.Background()`.
- `Server.Start` joins the error of `Shutdown` to the listener error.
- Open question: the TUI drops the cursor blink command after the user picks a mention. The cursor then stops until the next key. Phase 3 keeps this.
- `make check`, `make test-integration` and `make test-keto` pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
