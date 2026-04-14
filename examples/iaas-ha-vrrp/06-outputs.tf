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

output "example01-wan-ip" {
  value = stackit_public_ip.example01-wan.ip
}

output "example01-master-server-id" {
  value = stackit_server.example01.server_id
}

output "example02-wan-ip" {
  value = stackit_public_ip.example02-wan.ip
}

output "example02-backup-server-id" {
  value = stackit_server.example02.server_id
}

output "vip01-wan-ip" {
  value = stackit_public_ip.vip01-wan.ip
}

output "vip01-lan-ip" {
  value = stackit_network_interface.vip01.ipv4
}

output "vip01-network-interface" {
  value = stackit_network_interface.vip01.network_interface_id
}

output "default-network-id" {
  value = stackit_network.default.network_id
}
