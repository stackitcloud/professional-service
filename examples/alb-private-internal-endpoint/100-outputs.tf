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

output "alb_private_address" {
  description = "Private IPv4 address of the load balancer in the listener network. The load balancer has no public address."
  value       = stackit_application_load_balancer.this.private_address
}

output "api_host" {
  description = "Hostname of the internal API, carried by the listener certificate."
  value       = local.api_host
}

output "jumphost_public_ip" {
  description = "Public IPv4 address of the jump host."
  value       = stackit_public_ip.jumphost.ip
}

output "ssh_command" {
  description = "SSH command for the jump host (Debian default user)."
  value       = "ssh debian@${stackit_public_ip.jumphost.ip}"
}

output "backend_private_ips" {
  description = "Fixed private IPv4 addresses of the backend VMs in the target network, keyed by backend index."
  value       = local.backend_ips
}

output "target_security_group" {
  description = "Security group exported by the load balancer that permits its traffic to the targets; assigned to the backend interfaces by this configuration."
  value       = stackit_application_load_balancer.this.target_security_group
}

output "load_balancer_security_group" {
  description = "Security group exported by the load balancer that its own interfaces carry; use it as remote group in own rules instead of assigning the target security group."
  value       = stackit_application_load_balancer.this.load_balancer_security_group
}

output "ca_certificate_pem" {
  description = "Certificate of the private CA that issued the listener and backend certificates. Clients trust this certificate to verify the load balancer."
  value       = tls_self_signed_cert.ca.cert_pem
}
