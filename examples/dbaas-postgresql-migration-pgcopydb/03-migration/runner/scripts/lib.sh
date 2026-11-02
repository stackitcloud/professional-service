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

# Shared by the numbered steps, not run on its own.
#
# Every step runs twice:
#
#   1. On the migration host, started by you (sudo ./10-preflight.sh appdb).
#      The block below starts the same script again in a throwaway container
#      from the runner image and hands over. Nothing else runs on the host.
#   2. In the runner container (IN_RUNNER=1). psql, pg_dump and pgcopydb come
#      from the image, the connection data from migration.env.
#
# Mounted into the container:
#   /scripts          the steps, read-only
#   /sql              the SQL the steps run, read-only
#   /var/lib/pgcopydb logs, reports and the pgcopydb work directory, under the
#                     same path as on the host, so every path a step prints is
#                     valid on the host as well
set -Eeuo pipefail

RUNNER_IMAGE=localhost/pgcopydb-runner:latest
MIGRATION_DIR=/opt/migration
WORK=/var/lib/pgcopydb
SQL_DIR=/sql

#
# 1. On the host: hand over to the runner container.
#

if [[ -z "${IN_RUNNER:-}" ]]; then
    [[ $EUID -eq 0 ]] || exec sudo ALLOW_WRITES="${ALLOW_WRITES:-}" ASSUME_YES="${ASSUME_YES:-}" "$0" "$@"

    tty=()
    [[ -t 0 && -t 1 ]] && tty=(-it)
    # CA files of a real source with a private CA, see the README.
    certs=()
    [[ -d "$MIGRATION_DIR/certs" ]] && certs=(-v "$MIGRATION_DIR/certs:/certs:ro")

    exec podman run --rm "${tty[@]}" "${certs[@]}" \
        --network host \
        --env-file "$MIGRATION_DIR/migration.env" \
        -e IN_RUNNER=1 -e ALLOW_WRITES -e ASSUME_YES \
        -v "$WORK:$WORK" \
        -v "$MIGRATION_DIR/scripts:/scripts:ro" \
        -v "$MIGRATION_DIR/sql:$SQL_DIR:ro" \
        "$RUNNER_IMAGE" "/scripts/$(basename "$0")" "$@"
fi

#
# 2. In the runner container from here on.
#

export PGCONNECT_TIMEOUT=10

die() { echo "ERROR: $*" >&2; exit 1; }

# Sets DB, REPORTS and CLONE_DIR from the first argument of a step.
need_db() {
    DB="${1:-}"
    [[ -n "$DB" ]] || die "usage: $(basename "$0") <database>   (one of: $DATABASES)"
    [[ ",$DATABASES," == *",$DB,"* ]] || die "$DB is not in DATABASES ($DATABASES)"
    REPORTS="$WORK/reports/$DB"
    CLONE_DIR="$WORK/clone/$DB"
}

# The application users from migration.env, as an array and as a test.
IFS=, read -ra APP_USER_LIST <<<"${APP_USERS:-}"
is_app_user() { [[ ",${APP_USERS:-}," == *",$1,"* ]]; }

#
# Connections
#

urlencode() {
    local s="$1" out="" c i
    for ((i = 0; i < ${#s}; i++)); do
        c="${s:i:1}"
        case "$c" in
            [a-zA-Z0-9.~_-]) out+="$c" ;;
            *) printf -v c '%%%02X' "'$c"; out+="$c" ;;
        esac
    done
    printf '%s' "$out"
}

src_uri() {
    local uri="postgresql://$(urlencode "$SOURCE_USER"):$(urlencode "$SOURCE_PASSWORD")@$SOURCE_HOST:$SOURCE_PORT/$1?sslmode=$SOURCE_SSLMODE"
    [[ -n "${SOURCE_SSLROOTCERT:-}" ]] && uri+="&sslrootcert=$SOURCE_SSLROOTCERT"
    printf '%s' "$uri"
}

# verify-full: TLS, and the certificate must match the host name.
dst_uri() {
    printf '%s' "postgresql://$(urlencode "$TARGET_USER"):$(urlencode "$TARGET_PASSWORD")@$TARGET_HOST:$TARGET_PORT/$1?sslmode=verify-full&sslrootcert=system"
}

# psql against one database: stop at the first error, no headers, "|" between
# columns, so that the result can be read with  IFS='|' read -r a b c.
#   src_psql <database> -c "SELECT ..."
src_psql() { local db="$1"; shift; psql -X -v ON_ERROR_STOP=1 -qtAF'|' "$(src_uri "$db")" "$@"; }
dst_psql() { local db="$1"; shift; psql -X -v ON_ERROR_STOP=1 -qtAF'|' "$(dst_uri "$db")" "$@"; }

# The same for a file from sql/. Values go in as psql variables.
#   src_sql <database> fingerprint.sql -v app_users="$APP_USERS"
src_sql() { local db="$1" file="$2"; shift 2; src_psql "$db" -f "$SQL_DIR/$file" "$@"; }
dst_sql() { local db="$1" file="$2"; shift 2; dst_psql "$db" -f "$SQL_DIR/$file" "$@"; }

# Quoting for values that go into an SQL string in a script:
#   lit  -> 'value'   (a literal)
#   ident -> "name"   (a table, schema or role name)
lit()   { local q="'"; printf "'%s'" "${1//$q/$q$q}"; }
ident() { local q='"'; printf '"%s"' "${1//$q/$q$q}"; }

#
# Shared queries, the SQL is in sql/
#

# Normalized description of a database, see sql/fingerprint.sql.
#   fingerprint src|dst <database>
fingerprint() { "${1}_sql" "$2" fingerprint.sql -v app_users="${APP_USERS:-}"; }

# Roles the source database refers to, one per line, see sql/referenced-roles.sql.
#   referenced_roles <database>
referenced_roles() { src_sql "$1" referenced-roles.sql; }
