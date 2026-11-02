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

# checks source and target before anything is written. Read-only on
# both sides, safe to repeat at any time.
# Usage:  sudo ./10-preflight.sh <database>
#
# One function per section, called in order at the end of the file. Every
# check prints PASS, WARN, FAIL or INFO. Any FAIL ends with exit code 1.
source "$(dirname "$0")/lib.sh"
need_db "$@"

PASS=0 WARN=0 FAIL=0
pass() { PASS=$((PASS + 1)); echo "  [PASS] $*"; }
warn() { WARN=$((WARN + 1)); echo "  [WARN] $*"; }
fail() { FAIL=$((FAIL + 1)); echo "  [FAIL] $*"; }
info() { echo "  [INFO] $*"; }
hint() { echo "         $*"; }
section() { echo; echo "== $*"; }

summary() {
    echo
    echo "Preflight $DB: $PASS passed, $WARN warnings, $FAIL failed"
    if (( FAIL > 0 )); then
        echo "Fix the failures before 20-prepare-target."
        exit 1
    fi
    echo "Next: ./20-prepare-target.sh $DB"
    exit 0
}

check_connectivity() {
    section "Connectivity"
    local out

    if out=$(src_psql "$DB" -c 'SELECT 1' 2>&1); then
        pass "source reachable"
    else
        fail "source not reachable: $(tail -1 <<<"$out")"
    fi

    if out=$(dst_psql "$DB" -c 'SELECT 1' 2>&1); then
        pass "target reachable, TLS certificate verified (sslmode=verify-full)"
    else
        fail "target not reachable: $(tail -1 <<<"$out")"
        hint "timeout: the enterprise firewall blocks 5432 (rule from 03-migration)"
        hint "         or the ACL of the instance misses this host (02-postgres-flex)"
        hint "database \"$DB\" does not exist: 02-postgres-flex creates it"
    fi

    # Nothing after this makes sense without both connections.
    (( FAIL == 0 )) || summary
}

check_versions() {
    section "Versions"
    local src_major dst_major dump_major
    src_major=$(( $(src_psql "$DB" -c 'SHOW server_version_num') / 10000 ))
    dst_major=$(( $(dst_psql "$DB" -c 'SHOW server_version_num') / 10000 ))
    dump_major=$(pg_dump --version | grep -oE '[0-9]+' | head -1)

    if (( dst_major >= src_major )); then
        pass "source PostgreSQL $src_major, target $dst_major"
    else
        fail "target $dst_major is older than source $src_major: a downgrade does not work with pg_dump"
    fi

    if (( dump_major >= src_major )); then
        pass "pg_dump $dump_major in the runner can read source $src_major"
    else
        fail "pg_dump $dump_major is older than source $src_major: rebuild the runner with a newer PG_MAJOR"
    fi
}

check_encryption() {
    section "Encryption"
    local ssl version addr

    IFS='|' read -r ssl version addr <<<"$(dst_psql "$DB" -c "
        SELECT s.ssl, coalesce(s.version, '-'), coalesce(host(inet_client_addr()), '-')
        FROM pg_stat_ssl s WHERE s.pid = pg_backend_pid()")"
    if [[ "$ssl" == t ]]; then
        pass "target connection uses $version"
    else
        fail "target connection is not encrypted"
    fi
    info "the target sees this host as $addr"

    case "$SOURCE_HOST" in
        127.0.0.1 | localhost | ::1)
            info "source is local to this host, TLS not needed"
            return
            ;;
    esac
    IFS='|' read -r ssl version <<<"$(src_psql "$DB" -c "
        SELECT s.ssl, coalesce(s.version, '-')
        FROM pg_stat_ssl s WHERE s.pid = pg_backend_pid()")"
    if [[ "$ssl" == t ]]; then
        pass "source connection uses $version (sslmode=$SOURCE_SSLMODE)"
    else
        warn "source connection is not encrypted: set SOURCE_SSLMODE=verify-full"
    fi
}

# en_US.utf8 and en_US.UTF-8 are the same locale: glibc normalizes the
# codeset part. Compare in lower case and without "-".
norm_locale() { local l="${1,,}"; printf '%s' "${l//-/}"; }

check_target_database() {
    section "Target database"
    local n
    n=$(dst_sql "$DB" user-relations.sql)
    if (( n == 0 )); then
        pass "target database $DB is empty"
    else
        fail "target database $DB already holds $n relations, probably from an earlier run"
        hint "./90-reset-target.sh $DB empties it"
    fi

    local s_enc s_coll s_ctype s_prov s_ver d_enc d_coll d_ctype d_prov d_ver
    IFS='|' read -r s_enc s_coll s_ctype s_prov s_ver <<<"$(src_sql "$DB" database-properties.sql)"
    IFS='|' read -r d_enc d_coll d_ctype d_prov d_ver <<<"$(dst_sql "$DB" database-properties.sql)"

    if [[ "$s_enc" == "$d_enc" ]]; then
        pass "encoding $s_enc on both sides"
    else
        fail "encoding differs: source $s_enc, target $d_enc"
    fi

    local s_locale d_locale
    s_locale="$(norm_locale "$s_coll")|$(norm_locale "$s_ctype")|$s_prov"
    d_locale="$(norm_locale "$d_coll")|$(norm_locale "$d_ctype")|$d_prov"
    if [[ "$s_locale" != "$d_locale" ]]; then
        warn "collation differs: source $s_coll/$s_ctype ($s_prov), target $d_coll/$d_ctype ($d_prov)"
        hint "sort order of text and ORDER BY results may change. Check with the application team."
        return
    fi
    pass "collation $s_coll on both sides"
    # Same locale, other library version: some strings may sort differently.
    # The indexes are built on the target, so they match its library.
    if [[ "$s_ver" != "$d_ver" ]]; then
        info "collation library version: source $s_ver, target $d_ver. Rare strings may sort differently."
    fi
}

check_extensions() {
    section "Extensions"
    local exts ext schema available trusted installed
    exts=$(src_sql "$DB" extensions.sql)
    if [[ -z "$exts" ]]; then
        pass "source uses no extensions"
        return
    fi

    while IFS='|' read -r ext schema; do
        IFS='|' read -r available trusted installed <<<"$(dst_sql "$DB" extension-on-target.sql -v ext="$ext")"
        if (( installed > 0 )); then
            pass "$ext is already installed on the target"
        elif (( available == 0 )); then
            fail "$ext is not available on PostgreSQL Flex"
        elif [[ "$trusted" != t ]]; then
            fail "$ext is available but not trusted: $TARGET_USER cannot create it, ask STACKIT support"
        elif [[ "$schema" != public ]]; then
            warn "$ext lives in schema $schema on the source, 20-prepare-target only handles public"
        else
            pass "$ext is available and trusted, 20-prepare-target creates it"
        fi
    done <<<"$exts"
}

check_roles() {
    section "Roles and privileges"
    info "everything on the target will belong to $TARGET_USER (--no-owner)"
    info "application users: ${APP_USERS:-none}. 50-post gives them the grants of the source role of the same name."

    if is_app_user "$TARGET_USER"; then
        fail "$TARGET_USER owns everything and must not be an application user as well"
    fi

    local user exists
    for user in "${APP_USER_LIST[@]}"; do
        exists=$(dst_psql "$DB" -c "SELECT count(*) FROM pg_roles WHERE rolname = $(lit "$user")")
        if (( exists > 0 )); then
            pass "application user $user exists on the target"
        else
            fail "application user $user is missing on the target: apply 02-postgres-flex"
        fi
    done

    local role
    while read -r role; do
        [[ -z "$role" || "$role" == "$SOURCE_USER" ]] && continue
        is_app_user "$role" && continue
        warn "the source grants rights to $role, which has no user on the target. 50-post skips them."
        hint "add $role to app_users in 02-postgres-flex if the application needs it"
    done <<<"$(referenced_roles "$DB")"
}

check_special_objects() {
    section "Objects that need attention"
    local unlogged matviews lobs
    IFS='|' read -r unlogged matviews lobs <<<"$(src_sql "$DB" special-objects.sql)"

    if (( unlogged > 0 )); then
        warn "$unlogged UNLOGGED table(s): copied, but on PostgreSQL Flex their content is lost on a failover"
    fi
    if (( matviews > 0 )); then
        info "$matviews materialized view(s): 50-post refreshes them if they are empty on the target"
    fi
    if (( lobs > 0 )); then
        info "$lobs large object(s): pgcopydb copies them, this can take long"
    fi
}

check_capacity_and_timing() {
    section "Capacity and timing"
    local bytes pretty
    IFS='|' read -r bytes pretty <<<"$(src_psql "$DB" -c "
        SELECT pg_database_size(current_database()), pg_size_pretty(pg_database_size(current_database()))")"

    # Fail above the storage of the target, warn above 60 % of it: the copy
    # writes WAL and the index builds need room on top of the data.
    local storage=$(( TARGET_STORAGE_GB * 1024 * 1024 * 1024 ))
    if (( bytes > storage )); then
        fail "source $DB has $pretty, more than the target storage of ${TARGET_STORAGE_GB} GB"
    elif (( bytes * 10 > storage * 6 )); then
        warn "source $DB has $pretty, target storage only ${TARGET_STORAGE_GB} GB: WAL and index builds need headroom"
    else
        pass "source $DB has $pretty, target storage ${TARGET_STORAGE_GB} GB"
    fi
    info "the storage is shared by all databases of the instance, plan for all of them"

    info "target backup runs at cron '$TARGET_BACKUP_SCHEDULE' (UTC), now it is $(date -u +%H:%M) UTC"

    local sessions read_only
    IFS='|' read -r sessions read_only <<<"$(src_psql "$DB" -c "
        SELECT count(*), current_setting('default_transaction_read_only')
        FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid()")"
    if [[ "$read_only" == on ]]; then
        info "source $DB is frozen (read-only), $sessions other session(s)"
    else
        info "source $DB is writable, $sessions other session(s). Freeze it right before 40-clone."
    fi
}

echo "Preflight for database $DB"
echo "  source $SOURCE_HOST:$SOURCE_PORT as $SOURCE_USER"
echo "  target $TARGET_HOST:$TARGET_PORT as $TARGET_USER"

check_connectivity
check_versions
check_encryption
check_target_database
check_extensions
check_roles
check_special_objects
check_capacity_and_timing
summary
