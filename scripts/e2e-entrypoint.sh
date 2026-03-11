#!/usr/bin/env bash
set -euo pipefail

CHIT_HOST="${CHIT_HOST:-localhost}"
CHIT_PORT="${CHIT_PORT:-8065}"
MAX_WAIT="${MAX_WAIT:-60}"

echo "Waiting for chitd at ${CHIT_HOST}:${CHIT_PORT}..."
elapsed=0
while ! curl -sf "http://${CHIT_HOST}:${CHIT_PORT}/api/v1/system/ping" >/dev/null 2>&1; do
    if [ "$elapsed" -ge "$MAX_WAIT" ]; then
        echo "ERROR: chitd not available after ${MAX_WAIT}s"
        exit 1
    fi
    sleep 1
    elapsed=$((elapsed + 1))
done
echo "chitd is up (${elapsed}s)."

echo "Running e2e tests..."
go test -tags e2e -v -count=1 ./tests/e2e/...
