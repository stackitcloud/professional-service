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

output "stackit_project_id" {
  description = "The ID of the created STACKIT project"
  value       = stackit_resourcemanager_project.this.project_id
}

output "ske_node_network_id" {
  description = "The node network ID passed to the ALB controller via cloud.yaml"
  value       = stackit_network.ske_nodes.network_id
}

output "waf_configuration_name" {
  description = "The WAF configuration name attached to the ALB listener via Ingress annotation"
  value       = stackit_alb_waf_configuration.waf.name
}

output "alb_public_ip" {
  description = "The public IP of the ALB provisioned by the controller for the demo Ingress"
  value       = try(kubernetes_ingress_v1.hello.status[0].load_balancer[0].ingress[0].ip, null)
}

output "test_https_command" {
  description = "Verifies TLS termination on the ALB (-k skips trust validation for the self-signed cert)"
  value       = "curl -sfk --resolve ${var.test_app_hostname}:443:${try(kubernetes_ingress_v1.hello.status[0].load_balancer[0].ingress[0].ip, "<ALB_IP>")} https://${var.test_app_hostname}/"
}

output "test_waf_query_param" {
  description = "Triggers the WAF query-parameter block rule — expects HTTP 403"
  value       = "curl -sk -o /dev/null -w '%%{http_code}' --resolve ${var.test_app_hostname}:443:${try(kubernetes_ingress_v1.hello.status[0].load_balancer[0].ingress[0].ip, "<ALB_IP>")} 'https://${var.test_app_hostname}/?waf_test=trigger'"
}

output "test_waf_header" {
  description = "Triggers the WAF custom-header block rule — expects HTTP 403"
  value       = "curl -sk -o /dev/null -w '%%{http_code}' -H 'X-WAF-Test: trigger' --resolve ${var.test_app_hostname}:443:${try(kubernetes_ingress_v1.hello.status[0].load_balancer[0].ingress[0].ip, "<ALB_IP>")} 'https://${var.test_app_hostname}/'"
}
