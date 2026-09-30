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

output "ske_cluster_name" {
  description = "Name of the SKE cluster. Appears as the 'service_instance_id' label on every audit record."
  value       = stackit_ske_cluster.this.name
}

output "telemetry_router_id" {
  description = "ID of the Telemetry Router instance"
  value       = stackit_telemetryrouter_instance.this.instance_id
}

output "telemetry_router_uri" {
  description = "OTLP ingest URI of the Telemetry Router"
  value       = stackit_telemetryrouter_instance.this.uri
}

output "telemetry_link_id" {
  description = "ID of the Telemetry Link that subscribes the project stream to the router"
  value       = stackit_telemetrylink.this.id
}

output "grafana_url" {
  description = "Grafana URL of the Observability instance - this is where you run the LogQL queries from the README"
  value       = stackit_observability_instance.audit.grafana_url
}

output "canary_namespace" {
  description = "Namespace created through the ephemeral kubeconfig. Its creation shows up in the audit stream under your own identity."
  value       = kubernetes_namespace_v1.audit_canary.metadata[0].name
}

output "get_kubeconfig_command" {
  description = "Fetches a kubeconfig outside of Terraform, e.g. for the manual Secret verification steps in the README"
  value       = "stackit ske kubeconfig create ${stackit_ske_cluster.this.name} --project-id ${var.stackit_project_id}"
}
