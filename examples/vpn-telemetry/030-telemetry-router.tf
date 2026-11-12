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

# Router, Observability and the link live in the project of the VPN gateway.
# With your own VPN, put the ID of your project here.
locals {
  project_id = module.stackit.project_id
}

resource "stackit_telemetryrouter_instance" "this" {
  project_id   = local.project_id
  display_name = "${var.name_prefix}-router"
  description  = "Receives the telemetry of the linked projects"
}

resource "stackit_telemetryrouter_access_token" "this" {
  project_id   = local.project_id
  instance_id  = stackit_telemetryrouter_instance.this.instance_id
  display_name = "${var.name_prefix}-link-token"
}
