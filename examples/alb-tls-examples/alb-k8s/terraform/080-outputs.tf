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

output "project_id" {
  value       = stackit_resourcemanager_project.this.project_id
  description = "STACKIT Project ID. Required for kubernetes/ manifests and cert-manager ClusterIssuer."
}

output "cluster_name" {
  value       = stackit_ske_cluster.this.name
  description = "Name of the SKE cluster."
}

output "dns_zone_id" {
  value       = stackit_dns_zone.this.zone_id
  description = "STACKIT DNS Zone ID."
}

output "dns_zone_fqdn" {
  value       = var.dns_zone_fqdn
  description = "DNS zone FQDN. The app is reachable at <app_hostname>.<dns_zone_fqdn>."
}

output "app_fqdn" {
  value       = "${var.app_hostname}.${var.dns_zone_fqdn}"
  description = "Fully qualified domain name of the nginx showcase app."
}

output "kubeconfig_path" {
  value       = local_sensitive_file.kubeconfig.filename
  description = "Path to the generated kubeconfig."
}

output "next_steps" {
  value       = <<-EOT

    ✅ Infrastructure deployed.

    1. Set kubeconfig:
       export KUBECONFIG=$(terraform output -raw kubeconfig_path)

    2. Verify cluster:
       kubectl get nodes

    3. Note project_id for cert-manager config:
       terraform output project_id

    4. Deploy cluster components (sets DNS A record automatically):
       cd .. && bash scripts/deploy.sh
  EOT
  description = "Next steps after terraform apply."
}
