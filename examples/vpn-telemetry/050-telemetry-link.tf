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

resource "stackit_telemetrylink" "stackit" {
  resource_type           = "project"
  resource_id             = local.project_id
  display_name            = "${var.name_prefix}-stackit"
  telemetry_router_id     = stackit_telemetryrouter_instance.this.instance_id
  access_token_wo         = stackit_telemetryrouter_access_token.this.access_token
  access_token_wo_version = 1

  depends_on = [stackit_telemetryrouter_destination.observability]
}
