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

# The Secrets Manager instance unwraps the KMS key through this service
# account. For that, "kms.reader" role is currently required.
resource "stackit_service_account" "secrets_manager_kms" {
  project_id = var.stackit_project_id
  name       = var.name_prefix
}

resource "stackit_authorization_project_role_assignment" "secrets_manager_kms" {
  resource_id = var.stackit_project_id
  role        = "kms.reader"
  subject     = stackit_service_account.secrets_manager_kms.email
}
