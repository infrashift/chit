---
x-okfw-id: "01a1f633-2767-7b00-9fdd-5403454635c2"
title: "ISSUE-015: make kube-up waits forever on an old PostgreSQL volume"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "bug"
  - "closed"
x-okfw-created: "2026-11-19T17:45:09-06:00"
x-okfw-timestamp: "2026-11-19T17:57:10-06:00"
description: "make kube-up and make uat-up hang without an error when the PostgreSQL volume is from an older image that runs as another UID."
---

# ISSUE-015: make kube-up waits forever on an old PostgreSQL volume

**Status/Phase:** Closed · Bug

## Description
main moved PostgreSQL to the trusted `postgresql17` image. This image runs as UID 26. A `chit-pgdata` volume from the DHI postgres image belongs to UID 70, so the new image cannot write to it. PostgreSQL stops with `mkdir: cannot create directory '/var/lib/pgsql/data': Permission denied`, and Podman restarts it in a loop.

The `wait-postgres` init container has no limit on its loop, so `make kube-up` waits forever and shows no error. On 2026-11-19 the full-stack test waited one hour before a person found the cause in the journal. The UAT stack and its `chit-uat-pgdata` volume have the same problem.

## Acceptance Criteria
- [x] `make kube-up` and `make uat-up` stop before Podman starts when the volume belongs to an older image. The message tells how to keep or remove the data.
- [x] The check passes for a volume that does not exist, an empty volume and a volume of UID 26.
- [x] The `wait-postgres` loop has a limit, so a PostgreSQL that cannot start fails `podman kube play`.
- [x] The docs and the CHANGELOG describe the check.

## Resolution
Done on 2026-11-19.

- `scripts/check-pgdata.sh <volume>` reads the owner of the volume in the user namespace of Podman. It stops with exit code 1 when the volume is not empty and UID 26 does not own it. The message tells how to dump and restore the data, or how to remove a test volume.
- The new targets `kube-preflight` and `uat-preflight` run the check before `kube-up` and `uat-up` build the images.
- `wait-postgres` in both kube files gives up after 90 tries (3 minutes) and names the PostgreSQL container in its message. This also applies R2 (loop bounds) to the deploy files.
- The CHANGELOG and the Podman kube page describe the check.

Tests:
- The script passes for a volume that does not exist, an empty volume and the volume of UID 26. It stops for a volume of UID 70, and it gives exit code 2 without an argument. shellcheck reports no findings.
- `make uat-up` on a `chit-uat-pgdata` volume of UID 70 stopped in less than 1 second, before the image build.
- A pod with the real `wait-postgres` container and no PostgreSQL failed after 180 seconds, and `podman kube play` gave exit code 125.
- `make check` passes. On a new volume, `make kube-up` starts the stack, and `tests/e2e/test_e2e.sh` passes 296 of 296 checks.

## Related
- [ISSUE-014](/issues/issue-014-merge-main-and-bring-its-new-code-under-the-gates--01a1f5dc77bc.md)
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [ISSUE-016](/issues/issue-016-a-duplicate-team-or-channel-name-gives-a-500--01a1f63e23c0.md)
