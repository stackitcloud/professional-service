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

# Custom role: read-only access to SKE clusters and Object Storage buckets.
# Only the exact permissions listed here are granted — no wildcard expansion.
resource "stackit_authorization_project_custom_role" "readonly" {
  resource_id = var.stackit_project_id
  name        = "demo.readonly"
  description = "Read-only access to SKE and Object Storage for the iam-custom-roles demo."
  permissions = [
    "ske.cluster.list",
    "ske.cluster.get",
    "ske.version.list",
    "objectstorage.bucket.list",
    "objectstorage.bucket.get",
  ]
}

resource "stackit_authorization_project_role_assignment" "this" {
  resource_id = var.stackit_project_id
  role        = stackit_authorization_project_custom_role.readonly.name
  subject     = stackit_service_account.this.email
}
