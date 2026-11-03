#!/usr/bin/env bash
#
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

# Creates a table in the PostgreSQL Flex instance created by this example,
# then verifies that the table exists.
#
# The connection details are not taken from any config file: the script
# logs in to the STACKIT Secrets Manager instance with the user Terraform
# created, reads the secret, and connects to the database with the
# credentials found in it.
#
# Prerequisites: an applied Terraform state in this directory, curl, jq,
# the psql client, and your egress IP part of `acls`.
#
# Usage: ./create-table-from-sm.sh

set -euo pipefail

tf_output() {
  terraform output -raw "$1"
}

SM_ADDRESS="$(tf_output sm_vault_address)"
SM_INSTANCE_ID="$(tf_output instance_id)"
SM_SECRET_NAME="$(tf_output sm_secret_name)"
SM_USERNAME="$(tf_output sm_username)"
SM_PASSWORD="$(tf_output sm_password)"

echo ">> Logging in to Secrets Manager as ${SM_USERNAME}"
LOGIN_RESPONSE="$(curl -sf -X POST -H "Content-Type: application/json" \
  -d "{\"password\":\"${SM_PASSWORD}\"}" \
  "${SM_ADDRESS}/v1/auth/userpass/login/${SM_USERNAME}")"
VAULT_TOKEN="$(jq -r '.auth.client_token' <<<"$LOGIN_RESPONSE")"
if [[ "$VAULT_TOKEN" == "null" || -z "$VAULT_TOKEN" ]]; then
  echo "error: Secrets Manager login failed" >&2
  exit 1
fi

echo ">> Reading secret ${SM_SECRET_NAME} from the instance"
SECRET_RESPONSE="$(curl -sf -H "X-Vault-Token: ${VAULT_TOKEN}" \
  "${SM_ADDRESS}/v1/${SM_INSTANCE_ID}/data/${SM_SECRET_NAME}")"

DB_HOST="$(jq -r '.data.data.host' <<<"$SECRET_RESPONSE")"
DB_PORT="$(jq -r '.data.data.port' <<<"$SECRET_RESPONSE")"
DB_NAME="$(jq -r '.data.data.dbname' <<<"$SECRET_RESPONSE")"
DB_USERNAME="$(jq -r '.data.data.username' <<<"$SECRET_RESPONSE")"
DB_PASSWORD="$(jq -r '.data.data.password' <<<"$SECRET_RESPONSE")"
TABLE_NAME="$(jq -r '.data.data.table' <<<"$SECRET_RESPONSE")"

for value in "$DB_HOST" "$DB_PORT" "$DB_NAME" "$DB_USERNAME" "$DB_PASSWORD" "$TABLE_NAME"; do
  if [[ "$value" == "null" || -z "$value" ]]; then
    echo "error: incomplete secret in ${SM_SECRET_NAME}" >&2
    exit 1
  fi
done
# The table name is interpolated into SQL; only allow safe identifiers.
if ! [[ "$TABLE_NAME" =~ ^[a-z_][a-z0-9_]*$ ]]; then
  echo "error: unexpected table name in secret: ${TABLE_NAME}" >&2
  exit 1
fi

run_psql() {
  PGPASSWORD="$DB_PASSWORD" PGCONNECT_TIMEOUT=15 psql \
    --host "$DB_HOST" --port "$DB_PORT" --dbname "$DB_NAME" --username "$DB_USERNAME" \
    --no-password --set ON_ERROR_STOP=1 "$@"
}

echo ">> Connecting to ${DB_USERNAME}@${DB_HOST}:${DB_PORT}/${DB_NAME}"

echo ">> Creating table ${TABLE_NAME}"
run_psql --quiet -v table_name="$TABLE_NAME" <<'SQL'
CREATE TABLE IF NOT EXISTS :table_name (
  id serial PRIMARY KEY,
  note text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO :table_name (note) VALUES ('created by create-table-from-sm.sh');
SQL

echo ">> Verifying that the table exists"
# psql :var expansion does not work inside single quotes or in -c arguments,
# so interpolate in bash; TABLE_NAME was validated above, so this is safe.
EXISTS="$(run_psql --quiet -At -c "SELECT to_regclass('public.${TABLE_NAME}') IS NOT NULL")"
if [[ "$EXISTS" != "t" ]]; then
  echo "error: table ${TABLE_NAME} was not found" >&2
  exit 1
fi
echo "table ${TABLE_NAME} exists"

echo ">> Current rows:"
run_psql -c "SELECT * FROM ${TABLE_NAME} ORDER BY id"
