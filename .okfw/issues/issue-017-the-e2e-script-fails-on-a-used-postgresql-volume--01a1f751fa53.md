---
id: "01a1f751-fa53-7bfb-b153-c968152b05d0"
title: "ISSUE-017: The e2e script fails on a used PostgreSQL volume"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "bug"
  - "closed"
created: "2026-11-19T22:58:27-06:00"
timestamp: "2026-11-19T23:02:53-06:00"
description: "tests/e2e/test_e2e.sh passes only on a new chit-pgdata volume, because scenarios 10 to 12 make people, a team and channels with names that a previous run already used."
---

# ISSUE-017: The e2e script fails on a used PostgreSQL volume

**Status/Phase:** Closed · Bug

## Description
`tests/e2e/test_e2e.sh` passes only on a new `chit-pgdata` volume. A second run on the same volume fails, because scenarios 10, 11 and 12 make objects with fixed names that the previous run already made:

- 10.2 expects that chit has no user for Erin. After one run, chit has the user `erin`, so 10.2 fails.
- 10.5 makes the team `halo-e2e`. A second run gets `409`, and all of the Erin and Frank steps in scenarios 10 and 11 fail after it.
- 12.3 makes the channel `hook-e2e` in `test-team`. 12.7 counts the channels with the name `hook-open`, and 12.12 counts the audit records for the channel of 12.3.

Scenario 2 already uses a team and channels again when they exist.

[ISSUE-016](/issues/issue-016-a-duplicate-team-or-channel-name-gives-a-500--01a1f63e23c0.md) found the problem. Before that fix, the second run also showed a `500`.

## Acceptance Criteria
- [x] Each run uses names that no previous run used for Erin, the HALO team and the channels of scenario 12.
- [x] Two runs of `tests/e2e/test_e2e.sh` on the same volume pass with no failures.
- [x] The e2e README tells that a run does not need a new volume.

## Resolution
Done on 2026-11-19.

- `tests/e2e/test_e2e.sh` sets a random `RUN_ID` of 8 hex digits at the start. Erin (`erin-<RUN_ID>`), Frank (`Frank.Only+halo-<RUN_ID>@test.local`), the HALO team (`halo-e2e-<RUN_ID>`) and the channels of scenario 12 (`hook-e2e-<RUN_ID>`, `hook-open-<RUN_ID>`) use it.
- Step 11.8 found Frank's channel by its name, and counted the files of the image in all channels. It now takes the channel ID from the response of 10.12, and counts only the two channels of the run.
- The e2e README tells that a run does not need a new volume.

Tests:
- With the script from main, a run on a used volume failed in scenario 10 and stopped in scenario 11 on a SQL error. 10.5 got `409` for `halo-e2e`, the fix of ISSUE-016.
- With the fix, two runs one after the other on the same volume passed 293 of 296 checks with no failures. The 3 skips are scenario 2, which uses its team and channels again.
- shellcheck reports no new findings. `make check` passes.

## Related
- [ISSUE-016](/issues/issue-016-a-duplicate-team-or-channel-name-gives-a-500--01a1f63e23c0.md)
- [ISSUE-015](/issues/issue-015-make-kube-up-waits-forever-on-an-old-postgresql-volume--01a1f6332767.md)
