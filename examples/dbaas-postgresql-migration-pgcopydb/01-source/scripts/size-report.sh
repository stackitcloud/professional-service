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

# Sizes of the logical databases and their schemas.
# Usage (on the source VM):  sudo /opt/pg-source/size-report.sh
set -Eeuo pipefail

pex() { podman exec -i pg-source psql -v ON_ERROR_STOP=1 -U postgres "$@"; }

echo "== Databases =="
pex -d postgres <<-'SQL'
	SELECT datname, pg_size_pretty(pg_database_size(datname)) AS size
	FROM pg_database WHERE datistemplate = false ORDER BY pg_database_size(datname) DESC;
SQL

for db in appdb analyticsdb; do
    echo "== $db: schemas =="
    # relkind 'r','m': partitions count on their own, the parent ('p') holds 0 bytes.
    pex -d "$db" <<-'SQL'
		SELECT n.nspname AS schema,
		       count(*)  AS relations,
		       pg_size_pretty(sum(pg_table_size(c.oid)))          AS data,
		       pg_size_pretty(sum(pg_indexes_size(c.oid)))        AS indexes,
		       pg_size_pretty(sum(pg_total_relation_size(c.oid))) AS total
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','m')
		  AND n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
		GROUP BY n.nspname ORDER BY sum(pg_total_relation_size(c.oid)) DESC;
	SQL
done
