---
x-okfw-id: "01a1f537-9e6c-7e21-98c9-9336dbd4289d"
title: "ISSUE-009: R5: assertion density"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-9"
  - "power-of-10"
  - "r5"
  - "closed"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T14:37:08-06:00"
description: "Add internal/assert to each module and reach an average of two assertions for each function."
---

# ISSUE-009: R5: assertion density

**Status/Phase:** Closed · Phase 9

## Description
Add internal/assert to each module and reach an average of two assertions for each function.

## Acceptance Criteria
- [x] Each module has `internal/assert` with tests.
- [x] semscope reports an average of 2 or more assertions for each function in each module.
- [x] `forbidigo` bans `panic` outside `internal/assert`.
- [x] Worker, pump and listener goroutines have a recover guard.
- [x] NASA-P10-05 is not omitted and the omission list is empty.
- [x] `make check`, `make test-integration` and `make test-e2e` pass.

## Resolution
Done on 2026-11-19 in Phase 9 ([ADR-0011](/decisions/adr-0011-a-failed-assertion-stops-one-unit-of-work-or-the-process--01a1f56cdcd3.md)).

- Each module has `internal/assert` with `That(cond, msg)`. semscope counts 3,234 assertions in 1,441 functions, an average of 2.24.
- Assertions check programmer errors only: dependencies that a constructor sets, values that an earlier step validated or clamped, and postconditions. No assertion reads request, database, file or terminal data. The agents removed the checks that could see input, for example the normal form of a tag name.
- `errlog.Recover` guards the goroutines for one unit of work. The bridge thread worker uses `recoverThread`. The central loops have no guard.
- `forbidigo` bans `panic` outside `internal/assert` and the test files.
- Bug fixes in the TUI: a selection in a channel without posts crashed the next render. A new view could put the cursor past the last post. The thread inbox kept an old team count. New tests cover the first fix and the tiny terminal sizes.
- semscope reports zero findings with all rules on, except the R3 deviation.
- `make check`, `make test-integration`, `make test-keto` and `make test-e2e` pass. The tests pass three times with `-race`. The TUI e2e suite passes against a live chitd (15 tests).
- `tests/e2e/test_e2e.sh` passes against the full stack: 217 of 217. The chitd log has no panic.
- [ISSUE-012](/issues/issue-012-validatetagname-cuts-the-reserved-prefix-at-the-wrong-byte--01a1f5857d5f.md) records a bug that the agents found in `ValidateTagName`.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
