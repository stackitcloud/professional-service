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
  description = "The network ID passed to the ALB controller (cloud.yaml -> applicationLoadBalancer.networkId)"
  value       = stackit_network.ske_nodes.network_id
}

output "alb_public_ip" {
  description = "The public IP of the ALB provisioned by the controller for the demo IngressClass"
  value       = try(kubernetes_ingress_v1.hello.status[0].load_balancer[0].ingress[0].ip, null)
}

output "test_http_command" {
  description = "Verifies plain HTTP routing through the ALB"
  value       = "curl -sf --resolve ${var.test_app_hostname}:80:${try(kubernetes_ingress_v1.hello.status[0].load_balancer[0].ingress[0].ip, "<ALB_IP>")} http://${var.test_app_hostname}/"
}

output "test_https_command" {
  description = "Verifies TLS termination on the ALB with the self-signed demo certificate (-k skips trust validation)"
  value       = "curl -sfk --resolve ${var.test_app_hostname}:443:${try(kubernetes_ingress_v1.hello.status[0].load_balancer[0].ingress[0].ip, "<ALB_IP>")} https://${var.test_app_hostname}/"
}
