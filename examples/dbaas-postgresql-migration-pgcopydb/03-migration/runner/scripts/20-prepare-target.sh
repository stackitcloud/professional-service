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

# creates the extensions of the source on the target. pgcopydb runs
# with --skip-extensions afterwards, because without a superuser it cannot
# take them over as they are on the source.
# Usage:  sudo ./20-prepare-target.sh <database>
source "$(dirname "$0")/lib.sh"
need_db "$@"

exts=$(src_sql "$DB" extensions.sql)

if [[ -z "$exts" ]]; then
    echo "Source $DB uses no extensions, nothing to do."
fi

while IFS='|' read -r ext schema; do
    [[ -n "$ext" ]] || continue
    # pgcopydb creates every other schema itself and fails if it already exists.
    [[ "$schema" == public ]] || die "$ext lives in schema $schema. Create it on the target by hand after the schema exists."
    echo "CREATE EXTENSION $ext"
    dst_psql "$DB" -c "CREATE EXTENSION IF NOT EXISTS $(ident "$ext") WITH SCHEMA public"
done <<<"$exts"

dst_psql "$DB" -c "SELECT '  ' || extname || ' ' || extversion FROM pg_extension WHERE extname <> 'plpgsql' ORDER BY 1"
echo
echo "Next: ./30-freeze-source.sh $DB on"
