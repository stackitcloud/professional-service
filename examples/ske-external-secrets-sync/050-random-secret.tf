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

resource "random_password" "this" {
  length           = 16
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "vault_kv_secret_v2" "random_secret" {
  count               = 1
  mount               = stackit_secretsmanager_instance.instance.instance_id
  name                = "random-secret"
  cas                 = 1
  delete_all_versions = true
  data_json = jsonencode(
    {
      admin = random_password.this.result
    }
  )

  depends_on = [stackit_secretsmanager_user.user]
}
