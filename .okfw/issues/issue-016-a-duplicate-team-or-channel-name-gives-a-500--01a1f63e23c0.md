---
id: "01a1f63e-23c0-7933-9d6c-a2077bf4ab16"
title: "ISSUE-016: A duplicate team or channel name gives a 500"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "bug"
created: "2026-11-19T17:57:09-06:00"
timestamp: "2026-11-19T17:57:09-06:00"
description: "Creating a team, or a channel in a team, with a name that already exists gives a 500 instead of a 409, because the store does not map the unique violation."
---

# ISSUE-016: A duplicate team or channel name gives a 500

**Status/Phase:** Open · Bug

## Description
`SqlTeamStore.Save` and `SqlChannelStore.Save` do not map the Postgres error `23505` (a duplicate unique key) to a conflict error. When a team name or a channel name in a team already exists, the API gives `500 internal server error`. The user, post and tag stores and `SaveDirectChannel` give `409` for the same case with `isUniqueViolation`.

A second run of `tests/e2e/test_e2e.sh` on the same volume showed the problem on 2026-11-19. The chitd log had these lines:

- `createTeam`: `duplicate key value violates unique constraint "teams_name_key"`
- `createChannel`: `duplicate key value violates unique constraint "idx_channels_team_name_unique"`

main has the same code, so this branch did not cause the problem.

Also, `tests/e2e/test_e2e.sh` passes only on a new `chit-pgdata` volume. On a used volume, scenario 10 finds the user Erin from the previous run, and scenarios 10 and 11 fail.

## Acceptance Criteria
- [ ] A duplicate team name gives `409` with a store error ID.
- [ ] A duplicate channel name in a team gives `409` with a store error ID.
- [ ] Integration tests cover the two cases.
- [ ] The OpenAPI contract lists `409` for `POST /teams` and `POST /channels`.

## Related
- [ISSUE-015](/issues/issue-015-make-kube-up-waits-forever-on-an-old-postgresql-volume--01a1f6332767.md)
