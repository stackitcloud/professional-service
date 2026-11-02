#!/usr/bin/env bash
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
