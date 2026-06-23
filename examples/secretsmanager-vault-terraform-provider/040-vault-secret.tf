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

resource "vault_kv_secret_v2" "example" {
  mount               = stackit_secretsmanager_instance.example.instance_id
  name                = "my-secret"
  cas                 = 1
  delete_all_versions = true

  data_json = jsonencode(
    {
      grafana_password = stackit_observability_instance.example.grafana_initial_admin_password,
      other_secret     = "your-other-secret-value"
    }
  )
}
