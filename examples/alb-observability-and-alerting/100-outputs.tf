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

output "alb_external_address" {
  description = "Public IPv4 address of the Application Load Balancer."
  value       = stackit_application_load_balancer.this.external_address
}

output "alb_url" {
  description = "HTTPS URL of the load balancer. The certificate is self-signed, use curl -k."
  value       = "https://${stackit_application_load_balancer.this.external_address}"
}

output "backend_private_ips" {
  description = "Private IPv4 addresses of the backend VMs, keyed by backend index."
  value       = { for key, nic in stackit_network_interface.backend : key => nic.ipv4 }
}

output "observability_instance_id" {
  description = "ID of the Observability instance that receives the load balancer metrics and logs."
  value       = stackit_observability_instance.this.instance_id
}

output "grafana_url" {
  description = "Grafana of the Observability instance. Import dashboards/alb-overview.json here."
  value       = stackit_observability_instance.this.grafana_url
}

output "alerting_url" {
  description = "Alertmanager of the Observability instance."
  value       = stackit_observability_instance.this.alerting_url
}

output "metrics_url" {
  description = "Prometheus compatible query endpoint of the Observability instance."
  value       = stackit_observability_instance.this.metrics_url
}

output "logs_url" {
  description = "Loki compatible query endpoint of the Observability instance."
  value       = stackit_observability_instance.this.logs_url
}

output "observability_username" {
  description = "Technical user of the Observability instance, also valid for the query endpoints."
  value       = stackit_observability_credential.alb.username
}

output "observability_password" {
  description = "Password of the technical user of the Observability instance."
  value       = stackit_observability_credential.alb.password
  sensitive   = true
}
