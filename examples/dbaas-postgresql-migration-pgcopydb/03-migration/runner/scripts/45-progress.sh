#!/usr/bin/env bash
# Watch a running 40-clone from a second terminal: what pgcopydb writes to the
# target right now, and whether every worker connection is encrypted.
# Usage:  sudo ./45-progress.sh <database> [interval seconds]    (Ctrl-C to stop)
source "$(dirname "$0")/lib.sh"
need_db "$@"
interval="${2:-5}"

while true; do
    printf '\033[2J\033[H'
    echo "$(date -u +%H:%M:%S) UTC  $TARGET_HOST/$DB  (every ${interval}s, Ctrl-C to stop)"
    # Aligned output with headers, unlike src_sql/dst_sql: this is for reading.
    psql -X -q "$(dst_uri "$DB")" -f "$SQL_DIR/progress.sql"
    sleep "$interval"
done
