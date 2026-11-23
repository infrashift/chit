---
x-okfw-id: "01a1f5dc-77bc-7dd2-9a68-d82d498a36ae"
title: "ISSUE-014: Merge main and bring its new code under the gates"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-11"
  - "power-of-10"
  - "ste"
  - "closed"
x-okfw-created: "2026-11-19T16:10:28-06:00"
x-okfw-timestamp: "2026-11-19T17:35:32-06:00"
description: "Merge origin/main (attachments, the membership hook, the rate limits) and make its new code and docs pass every gate before the pull request."
---

# ISSUE-014: Merge main and bring its new code under the gates

**Status/Phase:** Closed · Phase 11

## Description
main gained PR #6 to PR #10 after this branch started. The merge commit `e1978f7` resolved 17 conflicted files. The new code of main does not meet the gates: semscope reported 56 findings, golangci-lint 79 issues, and lexis 89 findings. `internal/blob`, `internal/memberhook` and `pkg/imagecheck` have no assertions.

## Acceptance Criteria
- [x] semscope, golangci-lint and lexis report zero findings.
- [x] Each new package has an average of two or more assertions for each function.
- [x] The wiki describes the attachments, the membership hook and the new rate limits.
- [x] `make check`, `make test-integration`, `make test-keto`, `make test-e2e` and `tests/e2e/test_e2e.sh` pass.
- [x] The remote has the branch, and a pull request against main is open.

## Resolution
The merge commit `e1978f7` resolved 17 files. The new code of main now passes every gate:
- semscope reports 0 findings in 393 files, and golangci-lint reports 0 issues in both modules.
- The code has 3,485 assertions in 1,579 functions. `pkg/imagecheck` has its own `internal/assert`, because it must not import chit packages.
- `api.RateLimit` takes a `ClientAllowlist`, and `workers.NewFileSweeper` takes a `Sweeper` (R9).
- ADR-0012, ADR-0013 and ADR-0014 record the attachments, the membership hook and the rate limits.
- lexis reports 0 findings in 74 files.

Tests: `make check`, `make test-integration`, `make test-keto`, `make test-e2e` and the chit-tui e2e suite pass. `tests/e2e/test_e2e.sh` passes 296 of 296 checks, and the chitd log has no panic.

Upgrade note: the new `postgresql17` image runs as UID 26, and it cannot write to a `chit-pgdata` volume from the old image. The test stack removed that volume and made a new one.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
