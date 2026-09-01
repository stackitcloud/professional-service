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

resource "stackit_network" "this" {
  project_id       = var.stackit_project_id
  name             = "${var.name_prefix}-network"
  ipv4_prefix      = var.network_cidr
  ipv4_nameservers = ["1.1.1.1", "9.9.9.9"]
  labels           = local.labels
}

# The load balancer attaches its own target security group to the backend
# interfaces. That group only allows traffic from the load balancer, so the
# backends get a group of their own. A new security group permits all outbound
# traffic by default, which cloud-init needs to reach the metadata service.
resource "stackit_security_group" "backend" {
  project_id  = var.stackit_project_id
  name        = "${var.name_prefix}-backend"
  description = "Backend VMs of the ${var.name_prefix} load balancer"
  stateful    = true
  labels      = local.labels
}

resource "stackit_security_group_rule" "backend_http" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.backend.security_group_id
  direction         = "ingress"
  description       = "Application ports, reachable from inside the network"
  protocol          = { name = "tcp" }
  port_range        = { min = local.backend_port_min, max = local.backend_port_max }
  ip_range          = var.network_cidr
}
