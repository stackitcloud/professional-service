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


# Security group for the management interfaces: HTTPS (web UI) and SSH from var.mgmt_allowed_cidr
resource "stackit_security_group" "mgmt" {
  project_id  = local.project_id
  name        = "paloalto-mgmt"
  description = "Palo Alto management interfaces"
  stateful    = true
}

resource "stackit_security_group_rule" "mgmt_ingress" {
  for_each          = toset(["22", "443"])
  project_id        = local.project_id
  security_group_id = stackit_security_group.mgmt.security_group_id
  direction         = "ingress"
  ip_range          = var.mgmt_allowed_cidr
  protocol = {
    name = "tcp"
  }
  port_range = {
    min = tonumber(each.value)
    max = tonumber(each.value)
  }
}

# HA1 (control link) runs over the management interfaces: allow all traffic between them
resource "stackit_security_group_rule" "mgmt_ha1" {
  project_id               = local.project_id
  security_group_id        = stackit_security_group.mgmt.security_group_id
  direction                = "ingress"
  remote_security_group_id = stackit_security_group.mgmt.security_group_id
}

# Security group for the WAN interfaces: everything in, the firewall filters.
# Egress is open through the STACKIT default egress rules.
resource "stackit_security_group" "wan" {
  project_id  = local.project_id
  name        = "paloalto-wan"
  description = "Palo Alto WAN interfaces, open; filtering by PAN-OS"
  stateful    = true
}

resource "stackit_security_group_rule" "wan_ingress" {
  project_id        = local.project_id
  security_group_id = stackit_security_group.wan.security_group_id
  direction         = "ingress"
  ip_range          = "0.0.0.0/0"
}
