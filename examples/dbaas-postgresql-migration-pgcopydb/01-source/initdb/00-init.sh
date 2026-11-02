#!/usr/bin/env bash
# Runs once, on the first start of the container (empty $PGDATA).
set -Eeuo pipefail

SQL_DIR="/docker-entrypoint-initdb.d/sql"
psql_super() { psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" "$@"; }

echo "==> Roles"
psql_super --dbname postgres <<-'SQL'
	-- Roles are cluster-wide. pg_dump of a single database does not carry them,
	-- and PostgreSQL Flex does not let the customer create them via SQL. That is
	-- exactly why they are here: grants to these roles are what the migration
	-- has to deal with. Dummy passwords, the DBMS only listens on 127.0.0.1.
	CREATE ROLE app_rw LOGIN PASSWORD 'app_rw' CONNECTION LIMIT 20;
	CREATE ROLE app_ro LOGIN PASSWORD 'app_ro' CONNECTION LIMIT 20;
SQL

echo "==> Databases"
psql_super --dbname postgres <<-'SQL'
	CREATE DATABASE appdb       OWNER postgres ENCODING 'UTF8' LC_COLLATE 'en_US.utf8' LC_CTYPE 'en_US.utf8' TEMPLATE template0;
	CREATE DATABASE analyticsdb OWNER postgres ENCODING 'UTF8' LC_COLLATE 'en_US.utf8' LC_CTYPE 'en_US.utf8' TEMPLATE template0;
	COMMENT ON DATABASE appdb       IS 'OLTP example';
	COMMENT ON DATABASE analyticsdb IS 'Analytics example, small';
SQL

echo "==> Schema appdb"
psql_super --dbname appdb -f "$SQL_DIR/appdb.sql"

echo "==> Schema analyticsdb"
psql_super --dbname analyticsdb -f "$SQL_DIR/analyticsdb.sql"

echo "==> Init done"
