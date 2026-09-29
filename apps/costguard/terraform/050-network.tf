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


# A small network of its own; the server reaches the internet through the
# network's router (SNAT), so it needs no public IP. The first network of a
# project gets a router IP, billed like a public IP.
resource "stackit_network" "costguard" {
  count = local.create_network ? 1 : 0

  project_id       = local.project_id
  name             = "costguard"
  ipv4_prefix      = var.network_ipv4_prefix
  ipv4_nameservers = var.dns_nameservers
  labels           = local.labels
}

# Egress only. A new security group already allows all outbound IPv4 and
# IPv6 traffic (spike); nothing comes in, except SSH from one CIDR while
# break_glass is set.
resource "stackit_security_group" "costguard" {
  project_id  = local.project_id
  name        = "costguard"
  description = "costguard: egress only"
  stateful    = true
  labels      = local.labels
}

resource "stackit_security_group_rule" "break_glass_ssh" {
  count = var.break_glass == null ? 0 : 1

  project_id        = local.project_id
  security_group_id = stackit_security_group.costguard.security_group_id
  description       = "costguard break-glass SSH"
  direction         = "ingress"
  ether_type        = "IPv4"
  ip_range          = var.break_glass.allowed_cidr
  protocol          = { name = "tcp" }
  port_range        = { min = 22, max = 22 }
}

# Security stays on (tf-az-flavor-test sets security = false; don't copy that).
resource "stackit_network_interface" "costguard" {
  project_id         = local.project_id
  network_id         = local.network_id
  name               = "costguard"
  security_group_ids = [stackit_security_group.costguard.security_group_id]
  labels             = local.labels
}

resource "stackit_public_ip" "break_glass" {
  count = var.break_glass == null ? 0 : 1

  project_id           = local.project_id
  network_interface_id = stackit_network_interface.costguard.network_interface_id
  labels               = local.labels
}
