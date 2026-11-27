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

# The instance is created in the given project, its secrets are encrypted
# with the customer-managed key.
resource "stackit_secretsmanager_instance" "this" {
  project_id = var.stackit_project_id
  name       = var.name_prefix
  acls       = var.acls

  kms_key = {
    key_id                = stackit_kms_key.this.key_id
    key_ring_id           = stackit_kms_keyring.this.keyring_id
    key_version           = var.kms_key_version
    service_account_email = stackit_service_account.secrets_manager_kms.email
  }
}
