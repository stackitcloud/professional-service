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
  name       = "prom-proxy"
  project_id = var.stackit_project_id
}

resource "time_rotating" "rotate" {
  rotation_days = 150
}

resource "stackit_service_account_key" "this" {
  project_id            = var.stackit_project_id
  service_account_email = stackit_service_account.this.email
  ttl_days              = 180

  rotate_when_changed = {
    rotation = time_rotating.rotate.id
  }
}

resource "stackit_authorization_project_role_assignment" "this" {
  resource_id = var.stackit_project_id
  role        = "prometheus-proxy.reader"
  subject     = stackit_service_account.this.email
}
