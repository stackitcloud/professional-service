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

resource "stackit_network" "cluster" {
  project_id       = var.stackit_project_id
  name             = "${var.cluster_name}-network"
  ipv4_prefix      = var.network_ipv4_prefix
  ipv4_nameservers = ["9.9.9.9", "1.1.1.1"]
  routed           = true
  labels           = local.common_labels
}

resource "stackit_security_group" "cluster" {
  project_id = var.stackit_project_id
  name       = "${var.cluster_name}-sg"
  labels     = local.common_labels
}

# Nodes need outbound internet for STEC registration (TCP 443) and image pulls.
resource "stackit_security_group_rule" "egress_tcp" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "egress"
  protocol          = { name = "tcp" }
}

resource "stackit_security_group_rule" "egress_udp" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "egress"
  protocol          = { name = "udp" }
}

resource "stackit_security_group_rule" "ingress_tcp" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  protocol          = { name = "tcp" }
}

resource "stackit_security_group_rule" "ingress_udp" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  protocol          = { name = "udp" }
}

# NICs are pre-created so the security group is attached before first boot.
resource "stackit_network_interface" "cp" {
  count              = var.cp_count
  project_id         = var.stackit_project_id
  network_id         = stackit_network.cluster.network_id
  security_group_ids = [stackit_security_group.cluster.security_group_id]
}

resource "stackit_network_interface" "worker" {
  count              = var.worker_count
  project_id         = var.stackit_project_id
  network_id         = stackit_network.cluster.network_id
  security_group_ids = [stackit_security_group.cluster.security_group_id]
}
