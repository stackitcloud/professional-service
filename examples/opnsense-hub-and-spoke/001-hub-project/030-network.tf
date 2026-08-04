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

resource "stackit_routing_table" "rt_firewall_lan" {
  network_area_id = stackit_network_area.sna.network_area_id
  organization_id = var.stackit_organization_id
  name            = "rt_firewall_lan"
  system_routes   = true
}

resource "stackit_routing_table" "rt_firewall_wan" {
  network_area_id = stackit_network_area.sna.network_area_id
  organization_id = var.stackit_organization_id
  name            = "rt_firewall_wan"
  system_routes   = true
}

resource "stackit_network" "wan_network" {
  project_id       = local.hub_project_id
  name             = "wan-network"
  ipv4_nameservers = ["1.1.1.1", "9.9.9.9"]
  ipv4_prefix      = "10.28.0.0/24"
  routing_table_id = stackit_routing_table.rt_firewall_wan.routing_table_id
}

resource "stackit_network" "lan_network" {
  project_id       = local.hub_project_id
  name             = "lan-network"
  ipv4_nameservers = ["1.1.1.1", "9.9.9.9"]
  ipv4_prefix      = "10.28.1.0/24"
  routing_table_id = stackit_routing_table.rt_firewall_lan.routing_table_id
}

resource "stackit_network" "mgmt_network" {
  project_id       = local.hub_project_id
  name             = "mgmt-network"
  ipv4_nameservers = ["1.1.1.1", "9.9.9.9"]
  ipv4_prefix      = "10.28.2.0/24"
}

resource "stackit_network_interface" "nic_wan" {
  project_id = local.hub_project_id
  network_id = stackit_network.wan_network.network_id
  name       = "WAN_interface"
  security   = false
  ipv4       = "10.28.0.100"
}

resource "stackit_network_interface" "nic_lan" {
  project_id = local.hub_project_id
  network_id = stackit_network.lan_network.network_id
  name       = "LAN_interface"
  security   = false
  ipv4       = "10.28.1.100"
}

resource "stackit_security_group" "mgmt_sg" {
  project_id  = local.hub_project_id
  name        = "firewall-mgmt-sg"
  description = "Allow SSH and HTTPS to the hub firewall management interface"
}

resource "stackit_security_group_rule" "allow_ssh" {
  count = var.mgmt_ip_range != "" ? 1 : 0

  project_id        = local.hub_project_id
  security_group_id = stackit_security_group.mgmt_sg.security_group_id
  direction         = "ingress"
  protocol = {
    name = "tcp"
  }
  port_range = {
    min = 22
    max = 22
  }
  ip_range = var.mgmt_ip_range
}

resource "stackit_security_group_rule" "allow_http" {
  count = var.mgmt_ip_range != "" ? 1 : 0

  project_id        = local.hub_project_id
  security_group_id = stackit_security_group.mgmt_sg.security_group_id
  direction         = "ingress"
  protocol = {
    name = "tcp"
  }
  port_range = {
    min = 80
    max = 80
  }
  ip_range = var.mgmt_ip_range
}

resource "stackit_security_group_rule" "allow_https" {
  count = var.mgmt_ip_range != "" ? 1 : 0

  project_id        = local.hub_project_id
  security_group_id = stackit_security_group.mgmt_sg.security_group_id
  direction         = "ingress"
  protocol = {
    name = "tcp"
  }
  port_range = {
    min = 443
    max = 443
  }
  ip_range = var.mgmt_ip_range
}

resource "stackit_network_interface" "nic_mgmt" {
  project_id         = local.hub_project_id
  network_id         = stackit_network.mgmt_network.network_id
  name               = "MGMT_interface"
  security           = true
  ipv4               = "10.28.2.100"
  security_group_ids = [stackit_security_group.mgmt_sg.security_group_id]
}
