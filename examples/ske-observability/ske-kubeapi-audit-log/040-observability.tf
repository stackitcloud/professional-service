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

# Loki + Grafana in one managed package
resource "stackit_observability_instance" "audit" {
  project_id          = var.stackit_project_id
  name                = "${var.cluster_name}-audit"
  plan_name           = var.observability_plan_name
  logs_retention_days = var.observability_logs_retention_days
  alert_config        = null
  acl                 = var.telemetry_acl
}

# Credentials the Telemetry Router uses to push into the Observability OTLP endpoint.
resource "stackit_observability_credential" "router_ingest" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.audit.instance_id
}
