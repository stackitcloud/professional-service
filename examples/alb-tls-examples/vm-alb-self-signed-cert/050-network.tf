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

resource "stackit_network" "main" {
  project_id       = stackit_resourcemanager_project.showcase.project_id
  name             = "${var.name_prefix}-network"
  ipv4_prefix      = var.network_cidr
  ipv4_nameservers = ["8.8.8.8", "1.1.1.1"]
  routed           = true

  labels = local.common_labels
}

resource "stackit_security_group" "vm" {
  project_id  = stackit_resourcemanager_project.showcase.project_id
  name        = "${var.name_prefix}-vm-sg"
  description = "Security group for the showcase VM"
  stateful    = true

  labels = local.common_labels
}

resource "stackit_security_group_rule" "ssh_ingress" {
  project_id        = stackit_resourcemanager_project.showcase.project_id
  security_group_id = stackit_security_group.vm.security_group_id
  direction         = "ingress"
  description       = "SSH from admin CIDR only"

  protocol   = { name = "tcp" }
  port_range = { min = 22, max = 22 }
  ip_range   = var.admin_cidr
}

resource "stackit_security_group_rule" "http_ingress" {
  project_id        = stackit_resourcemanager_project.showcase.project_id
  security_group_id = stackit_security_group.vm.security_group_id
  direction         = "ingress"
  description       = "HTTP port 80 — ALB forwards traffic here after TLS termination"

  protocol   = { name = "tcp" }
  port_range = { min = 80, max = 80 }
  ip_range   = "0.0.0.0/0"
}

resource "stackit_security_group_rule" "egress_tcp" {
  project_id        = stackit_resourcemanager_project.showcase.project_id
  security_group_id = stackit_security_group.vm.security_group_id
  direction         = "egress"
  description       = "Allow all outbound TCP (Docker image pulls, apt)"

  protocol = { name = "tcp" }
  ip_range = "0.0.0.0/0"
}

resource "stackit_security_group_rule" "egress_udp" {
  project_id        = stackit_resourcemanager_project.showcase.project_id
  security_group_id = stackit_security_group.vm.security_group_id
  direction         = "egress"
  description       = "Allow all outbound UDP (DNS)"

  protocol = { name = "udp" }
  ip_range = "0.0.0.0/0"
}

resource "stackit_security_group_rule" "egress_icmp" {
  project_id        = stackit_resourcemanager_project.showcase.project_id
  security_group_id = stackit_security_group.vm.security_group_id
  direction         = "egress"
  description       = "Allow outbound ICMP"

  protocol = { name = "icmp" }
  ip_range = "0.0.0.0/0"
}
