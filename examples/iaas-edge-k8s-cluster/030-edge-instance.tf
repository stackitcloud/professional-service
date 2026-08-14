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

resource "stackit_edgecloud_instance" "this" {
  project_id   = var.stackit_project_id
  display_name = var.stec_instance_name
  plan_id      = var.stec_plan_id
}

resource "stackit_edgecloud_kubeconfig" "this" {
  project_id    = var.stackit_project_id
  instance_name = stackit_edgecloud_instance.this.display_name
  expiration    = 86400
}

# This token can be extracted from the state to access the UI
resource "stackit_edgecloud_token" "this" {
  project_id    = var.stackit_project_id
  instance_name = stackit_edgecloud_instance.this.display_name
}

# Kubeconfig is written to disk so local-exec scripts can use it.
resource "local_sensitive_file" "stec_kubeconfig" {
  content         = stackit_edgecloud_kubeconfig.this.kubeconfig
  filename        = "${path.module}/.stec.kubeconfig.json"
  file_permission = "0600"
}
