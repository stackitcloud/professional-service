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

# For demo purposes the router lives in the same project as the cluster. In a
# real setup you would run one router in a dedicated hub project
resource "stackit_telemetryrouter_instance" "this" {
  project_id   = var.stackit_project_id
  region       = var.stackit_region
  display_name = "${var.cluster_name}-router"
  description  = "Receives SKE kube-apiserver audit logs and STACKIT platform audit logs"
}

resource "stackit_telemetryrouter_access_token" "this" {
  project_id   = var.stackit_project_id
  instance_id  = stackit_telemetryrouter_instance.this.instance_id
  display_name = "${var.cluster_name}-link-token"
}
resource "stackit_telemetryrouter_destination" "observability_audit" {
  project_id   = var.stackit_project_id
  instance_id  = stackit_telemetryrouter_instance.this.instance_id
  display_name = "observability-kube-audit"
  description  = "Kubernetes audit records only"

  config = {
    config_type = "OpenTelemetry"

    # we can filter that we will only receive kubernetes audit logs from the specific cluster
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
      uri = stackit_observability_instance.audit.otlp_http_logs_url
      basic_auth = {
        username = stackit_observability_credential.router_ingest.username
        password = stackit_observability_credential.router_ingest.password
      }
    }
  }
}
