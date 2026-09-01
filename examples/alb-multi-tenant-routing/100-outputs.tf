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
  description = "Public IPv4 address of the Application Load Balancer, allocated by the load balancer itself."
  value       = stackit_application_load_balancer.this.external_address
}

output "app_host" {
  description = "Hostname of the web, api and canary applications."
  value       = local.app_host
}

output "admin_host" {
  description = "Hostname of the admin application."
  value       = local.admin_host
}

output "target_pools" {
  description = "Target pool names and the backend port each of them forwards to."
  value       = { for name, app in local.applications : local.pool_names[name] => app.port }
}

output "backend_private_ips" {
  description = "Private IPv4 addresses of the backend VMs, keyed by backend index."
  value       = { for key, nic in stackit_network_interface.backend : key => nic.ipv4 }
}
