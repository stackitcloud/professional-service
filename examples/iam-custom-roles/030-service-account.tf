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

resource "stackit_service_account" "this" {
  project_id = var.stackit_project_id
  name       = "iam-custom-roles-demo"
}

# Rotate the key every 80 days so it expires before the 90-day TTL.
resource "time_rotating" "key_rotation" {
  rotation_days = 80
}

resource "stackit_service_account_key" "this" {
  project_id            = var.stackit_project_id
  service_account_email = stackit_service_account.this.email
  ttl_days              = 90

  rotate_when_changed = {
    rotation = time_rotating.key_rotation.id
  }
}
