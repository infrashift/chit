---
id: "01a1f63e-23c0-7933-9d6c-a2077bf4ab16"
title: "ISSUE-016: A duplicate team or channel name gives a 500"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "bug"
  - "closed"
created: "2026-11-19T17:57:09-06:00"
timestamp: "2026-11-19T22:43:21-06:00"
description: "Creating a team, or a channel in a team, with a name that already exists gives a 500 instead of a 409, because the store does not map the unique violation."
---

# ISSUE-016: A duplicate team or channel name gives a 500

**Status/Phase:** Closed · Bug

## Description
`SqlTeamStore.Save` and `SqlChannelStore.Save` do not map the Postgres error `23505` (a duplicate unique key) to a conflict error. When a team name or a channel name in a team already exists, the API gives `500 internal server error`. The user, post and tag stores and `SaveDirectChannel` give `409` for the same case with `isUniqueViolation`.

A second run of `tests/e2e/test_e2e.sh` on the same volume showed the problem on 2026-11-19. The chitd log had these lines:

- `createTeam`: `duplicate key value violates unique constraint "teams_name_key"`
- `createChannel`: `duplicate key value violates unique constraint "idx_channels_team_name_unique"`

main has the same code, so this branch did not cause the problem.

Also, `tests/e2e/test_e2e.sh` passes only on a new `chit-pgdata` volume. On a used volume, scenario 10 finds the user Erin from the previous run, and scenarios 10 and 11 fail.

## Acceptance Criteria
- [x] A duplicate team name gives `409` with a store error ID.
- [x] A duplicate channel name in a team gives `409` with a store error ID.
- [x] Integration tests cover the two cases.
- [x] The OpenAPI contract lists `409` for `POST /teams` and `POST /channels`.

## Resolution
Done on 2026-11-19.

- `SqlTeamStore.Save` and `SqlChannelStore.Save` call `isUniqueViolation`. A taken name gives a conflict error (`409`) with the ID `SqlTeamStore.Save` or `SqlChannelStore.Save`. The app layer and the API pass the error on with no change.
- A deleted team keeps its name, because `teams.name` is `UNIQUE` on all rows. A channel name is unique only among the live channels of one team (`idx_channels_team_name_unique ... WHERE delete_at = 0`).
- `SqlTeamStore.Update` and `SqlChannelStore.Update` do not map the error. The API refuses a change to `name` with `400`, so an update cannot violate the index.
- The OpenAPI contract lists `409` for `POST /teams` and `POST /channels`. The CHANGELOG and the REST API page describe the fix.

Tests:
- `TestTeamStoreIntegration_SaveTakenNameIsConflict` checks the `409` and the ID for a taken name, and for the name of a deleted team.
- `TestChannelStoreIntegration_SaveTakenNameIsConflict` checks the `409` and the ID for a taken name in the same team. It also checks that another team can use the name, and that a new channel can use the name of a deleted channel.
- Without the store fix, the two tests fail with the two Postgres errors from the description.
- `make check` and `make test-integration` pass.

This issue does not change the e2e problem with the user Erin on a used volume.

## Related
- [ISSUE-015](/issues/issue-015-make-kube-up-waits-forever-on-an-old-postgresql-volume--01a1f6332767.md)
