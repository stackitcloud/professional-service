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

output "active01_wan_ip" {
  value = stackit_public_ip.active01_wan.ip
}

output "active01_server_id" {
  value = stackit_server.active01.server_id
}

output "passive02_wan_ip" {
  value = stackit_public_ip.passive02_wan.ip
}

output "passive02_passive_server_id" {
  value = stackit_server.passive02.server_id
}

output "vip01_wan_ip" {
  value = stackit_public_ip.vip01_wan.ip
}

output "vip01_lan_ip" {
  value = stackit_network_interface.vip01.ipv4
}

output "vip01_network_interface" {
  value = stackit_network_interface.vip01.network_interface_id
}

output "default_network_id" {
  value = stackit_network.default.network_id
}
