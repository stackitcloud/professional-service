#!/usr/bin/env bash
# Builds the runner image on the migration host. 03-migration calls it on apply.
# Usage:  sudo PG_MAJOR=18 ./00-build-runner.sh
set -Eeuo pipefail

[[ $EUID -eq 0 ]] || exec sudo PG_MAJOR="${PG_MAJOR:-18}" "$0" "$@"

PG_MAJOR="${PG_MAJOR:-18}"
DIR="$(cd "$(dirname "$0")/.." && pwd)"

podman build --build-arg PG_MAJOR="$PG_MAJOR" -t localhost/pgcopydb-runner:latest -f "$DIR/Containerfile" "$DIR"

pg_uid=$(podman run --rm localhost/pgcopydb-runner:latest id -u)
install -d -m 0750 -o "$pg_uid" -g root /var/lib/pgcopydb

# PostgreSQL 18 support came with pgcopydb 0.18 (large object API, pg_dump
# meta commands). An older package from the repository would fail mid-copy.
version=$(podman run --rm localhost/pgcopydb-runner:latest pgcopydb --version 2>&1 | grep -oE 'version [0-9]+\.[0-9]+' | head -1 | cut -d' ' -f2)
echo "pgcopydb $version, PostgreSQL client $PG_MAJOR"
if (( PG_MAJOR >= 18 )) && ! printf '0.18\n%s\n' "$version" | sort -V -C; then
    echo "ERROR: PostgreSQL $PG_MAJOR needs pgcopydb 0.18 or later, the image has $version" >&2
    exit 1
fi
