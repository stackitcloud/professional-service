#!/usr/bin/env bash
# Proves that the target matches the source.
#
# Without option: compares the target with the fingerprint 40-clone took from
# the frozen source. Objects, exact row counts, sequence values, constraints,
# triggers, functions and the privileges of the application users must be
# identical. Reads the catalogs and counts rows, fast.
#
# --compare-data: runs "pgcopydb compare data" instead. It reads every table
# on both sides in full and compares row count and a checksum of the content.
# Slow on large databases, and the source has to stay frozen until it is done.
#
# Usage:  sudo ./60-verify.sh <database>
#         sudo ./60-verify.sh <database> --compare-data
source "$(dirname "$0")/lib.sh"
need_db "$@"
mode="${2:-}"

verify_fingerprint() {
    [[ -s "$REPORTS/source.txt" ]] || die "no fingerprint of the source in $REPORTS, run 40-clone first"

    fingerprint dst "$DB" >"$REPORTS/target.txt"

    if diff -u "$REPORTS/source.txt" "$REPORTS/target.txt" >"$REPORTS/diff.txt"; then
        echo "IDENTICAL: $(grep -c '^obj' "$REPORTS/source.txt") objects, $(awk -F'|' '$1 == "rows" { s += $3 } END { print s + 0 }' "$REPORTS/source.txt") rows"
        echo
        echo "Database $DB is migrated. Cutover: point the application to"
        echo "  host=$TARGET_HOST port=$TARGET_PORT dbname=$DB sslmode=verify-full"
        echo "with one of the application users (${APP_USERS:-none}), not with $TARGET_USER,"
        echo "and keep the source frozen. Rollback: ./30-freeze-source.sh $DB off"
        echo
        echo "Optional, compares the content of every table: ./60-verify.sh $DB --compare-data"
    else
        cat "$REPORTS/diff.txt"
        echo
        die "source and target differ, see above and $REPORTS/diff.txt"
    fi
}

verify_data() {
    # The checksums are taken from the live source. If it is still written to,
    # they differ for reasons that have nothing to do with the copy.
    ro=$(src_psql "$DB" -c 'SHOW default_transaction_read_only')
    if [[ "$ro" != on && -z "${ALLOW_WRITES:-}" ]]; then
        die "source $DB is writable. ./30-freeze-source.sh $DB on, or ALLOW_WRITES=1 for a test run."
    fi

    # A fresh work directory: pgcopydb reuses the results of a completed run
    # in an existing one.
    local dir="$WORK/compare/$DB"
    rm -rf "$dir"
    mkdir -p "$REPORTS"

    # In the environment, so the passwords do not show up in the process list.
    export PGCOPYDB_SOURCE_PGURI PGCOPYDB_TARGET_PGURI
    PGCOPYDB_SOURCE_PGURI="$(src_uri "$DB")"
    PGCOPYDB_TARGET_PGURI="$(dst_uri "$DB")"

    echo "pgcopydb compare data     -> $REPORTS/compare-data.log"
    local start rc
    start=$(date +%s)
    set +e
    pgcopydb compare data --dir "$dir" 2>&1 | tee "$REPORTS/compare-data.log"
    rc=${PIPESTATUS[0]}
    set -e
    local duration=$(($(date +%s) - start))

    echo
    echo "Exit code $rc after $((duration / 60))m$((duration % 60))s"
    (( rc == 0 )) || die "data differs or the comparison failed, see $REPORTS/compare-data.log"
    echo "DATA IDENTICAL: row count and checksum of every table match."
}

case "$mode" in
    "") verify_fingerprint ;;
    --compare-data) verify_data ;;
    *) die "usage: $(basename "$0") <database> [--compare-data]" ;;
esac
