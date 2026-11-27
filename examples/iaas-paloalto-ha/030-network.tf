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

// All networks are inside the network area range 10.220.0.0/16 and use its nameservers (project/030-sna.tf).
// Only mgmt_network and wan_network are routed; HA and LAN traffic stays between the firewalls and their LAN peers.
locals {
  project_id = module.project.project_id

  // lan = true: the gateway announced by DHCP is the LAN IP of the active firewall (var.lan_gateway_host)
  networks = {
    mgmt = { prefix = "10.220.129.0/24", routed = true, lan = false }
    ha   = { prefix = "10.220.254.0/24", routed = false, lan = false }
    wan  = { prefix = "10.220.131.0/24", routed = true, lan = false }
    lan1 = { prefix = "10.220.1.0/24", routed = false, lan = true }
    lan2 = { prefix = "10.220.2.0/24", routed = false, lan = true }
  }

  vip = cidrhost(local.networks.wan.prefix, var.vip_host)

  lan_gateways = {
    for name, net in local.networks : name => cidrhost(net.prefix, var.lan_gateway_host) if net.lan
  }
}

resource "stackit_network" "net" {
  for_each     = local.networks
  project_id   = local.project_id
  name         = "${each.key}_network"
  ipv4_prefix  = each.value.prefix
  ipv4_gateway = lookup(local.lan_gateways, each.key, null)
  routed       = each.value.routed
}

// Management interfaces (PA "mgmt"), port security on, reachable via public IP
resource "stackit_network_interface" "mgmt" {
  for_each           = var.firewalls
  project_id         = local.project_id
  network_id         = stackit_network.net["mgmt"].network_id
  name               = "${each.key}-MGMT"
  ipv4               = cidrhost(local.networks.mgmt.prefix, each.value.host)
  security           = true
  security_group_ids = [stackit_security_group.mgmt.security_group_id]
}

// HA link (HA1/HA2), no port security
resource "stackit_network_interface" "ha" {
  for_each   = var.firewalls
  project_id = local.project_id
  network_id = stackit_network.net["ha"].network_id
  name       = "${each.key}-HA"
  ipv4       = cidrhost(local.networks.ha.prefix, each.value.host)
  security   = false
}

// WAN interfaces carry the VIP: port security must be on and the VIP must be an allowed address on
// both firewalls, otherwise the VIP port is not bound to the active firewall. 0.0.0.0/0 is needed in
// addition for the failover. The security group lets everything in, the PAN-OS security policies filter.
resource "stackit_network_interface" "wan" {
  for_each           = var.firewalls
  project_id         = local.project_id
  network_id         = stackit_network.net["wan"].network_id
  name               = "${each.key}-WAN"
  ipv4               = cidrhost(local.networks.wan.prefix, each.value.host)
  security           = true
  security_group_ids = [stackit_security_group.wan.security_group_id]
  allowed_addresses  = ["${local.vip}/32", "0.0.0.0/0"]
}

// LAN interfaces, no port security (the firewalls forward traffic for other addresses)
resource "stackit_network_interface" "lan1" {
  for_each   = var.firewalls
  project_id = local.project_id
  network_id = stackit_network.net["lan1"].network_id
  name       = "${each.key}-LAN1"
  ipv4       = cidrhost(local.networks.lan1.prefix, each.value.host)
  security   = false
}

resource "stackit_network_interface" "lan2" {
  for_each   = var.firewalls
  project_id = local.project_id
  network_id = stackit_network.net["lan2"].network_id
  name       = "${each.key}-LAN2"
  ipv4       = cidrhost(local.networks.lan2.prefix, each.value.host)
  security   = false
}

// VIP: a port that is never attached to a server. It reserves the address and carries the public IP,
// the active firewall takes it over with a gratuitous ARP.
resource "stackit_network_interface" "vip" {
  project_id = local.project_id
  network_id = stackit_network.net["wan"].network_id
  name       = "VIP"
  ipv4       = local.vip
  security   = false
}

resource "stackit_public_ip" "vip" {
  project_id           = local.project_id
  network_interface_id = stackit_network_interface.vip.network_interface_id
}

resource "stackit_public_ip" "mgmt" {
  for_each             = var.firewalls
  project_id           = local.project_id
  network_interface_id = stackit_network_interface.mgmt[each.key].network_interface_id
}
