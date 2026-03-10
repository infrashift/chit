#!/usr/bin/env bash
# uat-entrypoint.sh — wait for PostgreSQL, run migrations, seed UAT data, start chitd.
set -euo pipefail

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
MAX_WAIT="${MAX_WAIT:-30}"

echo "Waiting for PostgreSQL at ${DB_HOST}:${DB_PORT}..."
elapsed=0
while ! (echo >/dev/tcp/"${DB_HOST}"/"${DB_PORT}") 2>/dev/null; do
    if [ "$elapsed" -ge "$MAX_WAIT" ]; then
        echo "ERROR: PostgreSQL not available after ${MAX_WAIT}s"
        exit 1
    fi
    sleep 1
    elapsed=$((elapsed + 1))
done
echo "PostgreSQL is up (${elapsed}s)."

echo "Running migrations..."
migrate -database "${CHIT_DATABASE_URL}" -path /src/migrations up

echo "Seeding UAT data..."
go run /src/scripts/seed-uat/main.go

echo "Starting chitd..."
exec go run /src/cmd/chitd
