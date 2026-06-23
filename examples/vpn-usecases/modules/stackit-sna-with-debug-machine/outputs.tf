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

output "sna_id" {
  description = "The ID of the STACKIT Network Area."
  value       = stackit_network_area.this.network_area_id
}

output "project_id" {
  description = "The ID of the STACKIT project."
  value       = stackit_resourcemanager_project.this.project_id
}

output "machine_public_ip" {
  description = "The public IP address of the test machine."
  value       = stackit_public_ip.this.ip
}

output "machine_private_ipv4" {
  description = "The private IP address of the test machine."
  value       = stackit_network_interface.this.ipv4
}

output "machine_network_ipv4" {
  description = "The IPv4 prefix of the machine's network."
  value       = stackit_network.this.ipv4_prefix
}

output "sna_network_range" {
  description = "The network ranges (sna-ipv4) of the STACKIT Network Area."
  value       = stackit_network_area_region.this.ipv4.network_ranges
}
