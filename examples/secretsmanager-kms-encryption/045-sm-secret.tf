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

# Used by the vault provider to write the secret (hence write_enabled) and
# by the round-trip script to read it back.
resource "stackit_secretsmanager_user" "this" {
  project_id    = var.stackit_project_id
  instance_id   = stackit_secretsmanager_instance.this.instance_id
  description   = "User for Terraform and the download script"
  write_enabled = true
}

# The database credentials are stored in the instance. The round-trip
# script reads them from here, so no database secret has to live in a
# config file.
resource "vault_kv_secret_v2" "postgres_credentials" {
  mount               = stackit_secretsmanager_instance.this.instance_id
  name                = "${var.name_prefix}-postgres-credentials"
  cas                 = 1
  delete_all_versions = true

  data_json = jsonencode({
    host     = stackit_postgresflex_instance.this.connection_info.write.host
    port     = stackit_postgresflex_instance.this.connection_info.write.port
    dbname   = stackit_postgresflex_database.this.name
    username = stackit_postgresflex_user.this.username
    password = stackit_postgresflex_user.this.password
    table    = local.roundtrip_table
  })
}
