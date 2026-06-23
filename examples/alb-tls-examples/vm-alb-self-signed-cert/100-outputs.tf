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
  description = "UUID of the created project"
  value       = stackit_resourcemanager_project.showcase.project_id
}

output "alb_public_ip" {
  description = "Public IPv4 address of the Application Load Balancer"
  value       = stackit_public_ip.alb.ip
}

output "alb_https_url" {
  description = "HTTPS URL — certificate is self-signed, use curl -k or accept the browser warning"
  value       = "https://${stackit_public_ip.alb.ip}"
}

output "alb_http_url" {
  description = "Plain HTTP URL for baseline connectivity tests"
  value       = "http://${stackit_public_ip.alb.ip}"
}

output "vm_public_ip" {
  description = "Public IPv4 address of the VM (for SSH access)"
  value       = stackit_public_ip.vm.ip
}

output "vm_private_ip" {
  description = "Private IPv4 address of the VM"
  value       = stackit_network_interface.vm.ipv4
}

output "ssh_command" {
  description = "SSH command to connect to the VM (Debian 12 default user)"
  value       = "ssh debian@${stackit_public_ip.vm.ip}"
}

output "certificate_id" {
  description = "STACKIT ALB certificate ID referenced by the HTTPS listener"
  value       = stackit_alb_certificate.self_signed.cert_id
}

output "certificate_expiry" {
  description = "Certificate expiry timestamp (UTC)"
  value       = tls_self_signed_cert.self_signed.validity_end_time
}

output "dns_name" {
  description = "DNS name pointing to the ALB"
  value       = var.dns_name
}

output "dns_nameservers" {
  description = "Nameservers for the DNS zone — set these at your registrar to delegate the zone"
  value       = stackit_dns_zone.showcase.primary_name_server
}

output "curl_test" {
  description = "curl command to test the HTTPS endpoint by DNS name"
  value       = "curl -k https://${var.dns_name}"
}
