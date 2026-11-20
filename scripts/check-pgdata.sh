#!/bin/sh
# Usage: check-pgdata.sh <volume>
#
# Stops `make kube-up` and `make uat-up` before Podman starts a stack whose
# PostgreSQL cannot start. The trusted postgresql17 image runs as UID 26.
# A volume that an older image made (the DHI postgres image runs as UID 70)
# is not writable for it: PostgreSQL restarts in a loop, and the
# wait-postgres init container never sees it.
#
# The check passes when the volume does not exist (Podman makes it), when
# it is empty, or when UID 26 owns it.
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 <volume>" >&2
	exit 2
fi
vol=$1
pg_uid=26

podman volume exists "$vol" || exit 0
mp=$(podman volume inspect "$vol" --format '{{.Mountpoint}}')

# Rootless Podman maps the container UIDs into a user namespace: read the
# owner from inside it, so UID 26 shows as 26 and not as a subordinate UID.
in_ns() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	else
		podman unshare "$@"
	fi
}

if [ -z "$(in_ns find "$mp" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
	exit 0
fi
owner=$(in_ns stat -c %u "$mp")
if [ "$owner" = "$pg_uid" ]; then
	exit 0
fi

cat >&2 <<EOF
ERROR: the Podman volume '$vol' belongs to UID $owner, not UID $pg_uid.

An older PostgreSQL image made this volume. The postgresql17 image runs as
UID $pg_uid and cannot write to it: PostgreSQL would restart in a loop, and
the stack would never start.

- To keep the data, dump it with the old image and restore it into a new
  volume. See "Upgrading from the DHI PostgreSQL image" in
  docs/src/content/docs/deployment/podman-kube.mdx.
- If the volume holds only test data, remove it:

    podman volume rm $vol
EOF
exit 1
