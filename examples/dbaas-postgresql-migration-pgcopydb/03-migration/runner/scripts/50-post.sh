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

# What pgcopydb leaves to you. Refreshes materialized views that are
# empty on the target but populated on the source, and gives the application
# users the grants their source role had. --no-acl dropped them during the copy,
# because the owner on the target is a different user.
# Usage:  sudo ./50-post.sh <database>
source "$(dirname "$0")/lib.sh"
need_db "$@"

echo "== Materialized views"
mvs=$(src_sql "$DB" populated-matviews.sql)
[[ -n "$mvs" ]] || echo "  none populated on the source"
while IFS='|' read -r schema name; do
    [[ -n "$schema" ]] || continue
    mv="$(ident "$schema").$(ident "$name")"
    populated=$(dst_psql "$DB" -c "SELECT relispopulated FROM pg_class WHERE oid = $(lit "$mv")::regclass")
    if [[ "$populated" == t ]]; then
        echo "  $schema.$name already populated"
    else
        echo "  REFRESH MATERIALIZED VIEW $schema.$name"
        dst_psql "$DB" -c "REFRESH MATERIALIZED VIEW $mv"
    fi
done <<<"$mvs"

echo
echo "== Grants"
grants="$REPORTS/source-grants.sql"
applied="$REPORTS/applied-grants.sql"
if [[ ! -s "$grants" ]]; then
    echo "  the source had none"
else
    # One statement per line, as pg_dump writes them. The grantee is the last
    # word: "... TO app_rw;", "... FROM app_ro;", optionally WITH GRANT OPTION.
    re=' (TO|FROM) ("([^"]+)"|([A-Za-z0-9_]+))( WITH GRANT OPTION)?;$'
    : >"$applied"
    skipped=""
    while IFS= read -r stmt; do
        grantee=""
        [[ "$stmt" =~ $re ]] && grantee="${BASH_REMATCH[3]}${BASH_REMATCH[4]}"
        if [[ -z "$grantee" ]] || ! is_app_user "$grantee"; then
            skipped+="${grantee:-(not parsed)}"$'\n'
            continue
        fi
        # Default privileges were defined for the owner on the source. On the
        # target the owner user creates the objects, so they are defined for it.
        sed -E "s/FOR ROLE (\"[^\"]+\"|[^ ]+) /FOR ROLE $(ident "$TARGET_USER") /" <<<"$stmt" >>"$applied"
    done <"$grants"

    if [[ -s "$applied" ]]; then
        dst_psql "$DB" -1 -f "$applied"
        echo "  applied $(grep -c "" "$applied") statement(s) for ${APP_USERS//,/, }, see $applied"
    else
        echo "  no statement for ${APP_USERS:-the application users}"
    fi
    if [[ -n "$skipped" ]]; then
        echo "  skipped, no application user: $(sort -u <<<"${skipped%$'\n'}" | paste -sd, - | sed 's/,/, /g')"
        echo "  the statements stay in $grants"
    fi
fi

echo
echo "Next: ./60-verify.sh $DB"
