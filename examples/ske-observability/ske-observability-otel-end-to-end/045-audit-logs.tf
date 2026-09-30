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

resource "stackit_telemetryrouter_instance" "audit" {
  count = var.audit_logs_enabled ? 1 : 0

  project_id   = var.stackit_project_id
  region       = var.stackit_region
  display_name = "${var.cluster_name}-router"
}

resource "stackit_telemetryrouter_access_token" "audit" {
  count = var.audit_logs_enabled ? 1 : 0

  project_id   = var.stackit_project_id
  instance_id  = stackit_telemetryrouter_instance.audit[0].instance_id
  display_name = "${var.cluster_name}-link-token"
}

resource "stackit_observability_credential" "router_ingest" {
  count = var.audit_logs_enabled ? 1 : 0

  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.example.instance_id
}

resource "stackit_telemetryrouter_destination" "observability" {
  count = var.audit_logs_enabled ? 1 : 0

  project_id   = var.stackit_project_id
  instance_id  = stackit_telemetryrouter_instance.audit[0].instance_id
  display_name = "observability-kube-audit"
  description  = "Kubernetes audit records of ${var.cluster_name} only"

  config = {
    config_type = "OpenTelemetry"

    # The project stream also carries STACKIT platform audit logs and the
    # audit logs of other clusters. Only this cluster's kube-apiserver passes.
    filter = {
      attributes = [
        {
          key     = "stackit.log.kind"
          level   = "logRecord"
          matcher = "="
          values  = ["kubernetes-audit"]
        },
        {
          key     = "service.instance.id"
          level   = "logRecord"
          matcher = "="
          values  = [var.cluster_name]
        },
      ]
    }

    opentelemetry = {
      uri = stackit_observability_instance.example.otlp_http_logs_url
      basic_auth = {
        username = stackit_observability_credential.router_ingest[0].username
        password = stackit_observability_credential.router_ingest[0].password
      }
    }
  }
}

resource "stackit_telemetrylink" "audit" {
  count = var.audit_logs_enabled ? 1 : 0

  resource_type           = "project"
  resource_id             = var.stackit_project_id
  display_name            = "${var.cluster_name}-project-link"
  telemetry_router_id     = stackit_telemetryrouter_instance.audit[0].instance_id
  access_token_wo         = stackit_telemetryrouter_access_token.audit[0].access_token
  access_token_wo_version = var.telemetry_link_token_version
  enabled                 = true
}
