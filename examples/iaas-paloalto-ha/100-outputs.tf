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
  value = local.project_id
}

output "vip" {
  value = {
    private_ip = local.vip
    public_ip  = stackit_public_ip.vip.ip
  }
}

output "management" {
  value = {
    for name, ip in stackit_public_ip.mgmt : name => "https://${ip.ip}"
  }
}

output "interfaces" {
  description = "Private IPs per firewall, keyed by PA interface"
  value = {
    for name in keys(var.firewalls) : name => {
      "mgmt"             = stackit_network_interface.mgmt[name].ipv4
      "ethernet1/1 HA"   = stackit_network_interface.ha[name].ipv4
      "ethernet1/2 WAN"  = stackit_network_interface.wan[name].ipv4
      "ethernet1/3 LAN1" = stackit_network_interface.lan1[name].ipv4
      "ethernet1/4 LAN2" = stackit_network_interface.lan2[name].ipv4
    }
  }
}

output "lan_gateways" {
  description = "IPs to configure on ethernet1/3 and ethernet1/4 (synchronized, used by the active firewall)"
  value       = local.lan_gateways
}
