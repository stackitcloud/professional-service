#!/usr/bin/env bash
# Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Records the state of the frozen source first, then runs
# pgcopydb clone: schema, data in parallel COPY streams, indexes and
# constraints after the data, sequences, VACUUM ANALYZE.
#
# Usage:  sudo ./40-clone.sh <database>
#         sudo ALLOW_WRITES=1 ./40-clone.sh <database>   # test run without freeze
source "$(dirname "$0")/lib.sh"
need_db "$@"

[[ ! -e "$CLONE_DIR" ]] || die "$CLONE_DIR exists from an earlier run. ./90-reset-target.sh $DB starts over."

ro=$(src_psql "$DB" -c 'SHOW default_transaction_read_only')
if [[ "$ro" != on && -z "${ALLOW_WRITES:-}" ]]; then
    die "source $DB is writable. ./30-freeze-source.sh $DB on, or ALLOW_WRITES=1 for a test run."
fi

mkdir -p "$REPORTS"

echo "Fingerprint of the source -> $REPORTS/source.txt"
fingerprint src "$DB" >"$REPORTS/source.txt"

# --no-acl drops every GRANT. Keep them, so they can be re-applied to the
# users that exist on PostgreSQL Flex.
echo "Grants of the source      -> $REPORTS/source-grants.sql"
pg_dump --schema-only --no-owner -d "$(src_uri "$DB")" \
    | grep -E '^(GRANT|REVOKE|ALTER DEFAULT PRIVILEGES)' >"$REPORTS/source-grants.sql" || true

# In the environment, so the passwords do not show up in the process list.
export PGCOPYDB_SOURCE_PGURI PGCOPYDB_TARGET_PGURI
PGCOPYDB_SOURCE_PGURI="$(src_uri "$DB")"
PGCOPYDB_TARGET_PGURI="$(dst_uri "$DB")"

# --no-owner             the source owners do not exist on PostgreSQL Flex
# --no-acl               neither do the grantees, and the target user cannot create roles
# --skip-extensions      20-prepare-target created them already
# --skip-db-properties   ALTER DATABASE ... SET belongs to the instance, not to the copy
# --fail-fast            stop at the first error instead of leaving a half copy
echo "pgcopydb clone            -> $REPORTS/clone.log"
start=$(date +%s)
set +e
pgcopydb clone \
    --dir "$CLONE_DIR" \
    --no-owner \
    --no-acl \
    --skip-extensions \
    --skip-db-properties \
    --table-jobs "$TABLE_JOBS" \
    --index-jobs "$INDEX_JOBS" \
    --split-tables-larger-than "$SPLIT_TABLES_LARGER_THAN" \
    --fail-fast \
    2>&1 | tee "$REPORTS/clone.log"
rc=${PIPESTATUS[0]}
set -e
duration=$(($(date +%s) - start))

echo
echo "Exit code $rc after $((duration / 60))m$((duration % 60))s"
echo "ERROR/FATAL lines in the log: $(grep -cE 'ERROR|FATAL' "$REPORTS/clone.log" || true)"
(( rc == 0 )) || die "pgcopydb failed, see $REPORTS/clone.log. ./90-reset-target.sh $DB before the next try."
echo "Next: ./50-post.sh $DB"
