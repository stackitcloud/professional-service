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
