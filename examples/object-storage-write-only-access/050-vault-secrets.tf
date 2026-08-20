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

resource "vault_kv_secret_v2" "admin_creds" {
  mount               = stackit_secretsmanager_instance.this.instance_id
  name                = "s3-credentials/admin"
  cas                 = 1
  delete_all_versions = true

  data_json = jsonencode({
    access_key = stackit_objectstorage_credential.admin.access_key
    secret_key = stackit_objectstorage_credential.admin.secret_access_key
  })
}

resource "vault_kv_secret_v2" "write_creds" {
  mount               = stackit_secretsmanager_instance.this.instance_id
  name                = "s3-credentials/write"
  cas                 = 1
  delete_all_versions = true

  data_json = jsonencode({
    access_key = stackit_objectstorage_credential.write.access_key
    secret_key = stackit_objectstorage_credential.write.secret_access_key
  })
}

resource "vault_kv_secret_v2" "read_creds" {
  mount               = stackit_secretsmanager_instance.this.instance_id
  name                = "s3-credentials/read"
  cas                 = 1
  delete_all_versions = true

  data_json = jsonencode({
    access_key = stackit_objectstorage_credential.read.access_key
    secret_key = stackit_objectstorage_credential.read.secret_access_key
  })
}
