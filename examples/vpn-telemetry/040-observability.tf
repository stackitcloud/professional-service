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

resource "stackit_observability_instance" "this" {
  project_id          = local.project_id
  name                = var.name_prefix
  plan_name           = "Observability-Basic-EU01"
  logs_retention_days = 7
  alert_config        = null
  acl                 = ["0.0.0.0/0"]

  # sign in with STACKIT SSO, no default admin user
  grafana_admin_enabled = false
}

resource "stackit_observability_credential" "router" {
  project_id  = local.project_id
  instance_id = stackit_observability_instance.this.instance_id
}

resource "stackit_telemetryrouter_destination" "observability" {
  project_id   = local.project_id
  instance_id  = stackit_telemetryrouter_instance.this.instance_id
  display_name = "${var.name_prefix}-observability"
  description  = "VPN records to Observability"

  config = {
    config_type = "OpenTelemetry"

    filter = {
      attributes = [
        {
          key     = "service.name"
          level   = "logRecord"
          matcher = "="
          values  = ["vpn"]
        },
      ]
    }

    opentelemetry = {
      uri = stackit_observability_instance.this.otlp_http_logs_url
      basic_auth = {
        username = stackit_observability_credential.router.username
        password = stackit_observability_credential.router.password
      }
    }
  }
}
