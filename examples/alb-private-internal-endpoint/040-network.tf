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

# The listener of the load balancer and the jump host live in one network,
# the backends in another. Both are routed networks of the same project, so
# traffic between them is forwarded by the project router.
resource "stackit_network" "listener" {
  project_id       = var.stackit_project_id
  name             = "${var.name_prefix}-listener"
  ipv4_prefix      = var.listener_network_cidr
  ipv4_nameservers = ["1.1.1.1", "9.9.9.9"]
  labels           = local.labels
}

resource "stackit_network" "target" {
  project_id       = var.stackit_project_id
  name             = "${var.name_prefix}-target"
  ipv4_prefix      = var.target_network_cidr
  ipv4_nameservers = ["1.1.1.1", "9.9.9.9"]
  labels           = local.labels
}

resource "stackit_security_group" "jumphost" {
  project_id  = var.stackit_project_id
  name        = "${var.name_prefix}-jumphost"
  description = "Jump host of the ${var.name_prefix} example"
  stateful    = true
  labels      = local.labels
}

resource "stackit_security_group_rule" "jumphost_ssh" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.jumphost.security_group_id
  direction         = "ingress"
  description       = "SSH from the admin address range"
  protocol          = { name = "tcp" }
  port_range        = { min = 22, max = 22 }
  ip_range          = var.admin_cidr
}

# Traffic from the load balancer is permitted by the target security group
# that the load balancer exports (see 060-backends.tf). This group covers
# everything else the backends need: outbound traffic for cloud-init and
# direct access from the jump host for inspecting the backend certificates.
resource "stackit_security_group" "backend" {
  project_id  = var.stackit_project_id
  name        = "${var.name_prefix}-backend"
  description = "Backend VMs of the ${var.name_prefix} load balancer"
  stateful    = true
  labels      = local.labels
}

resource "stackit_security_group_rule" "backend_https_from_jumphost" {
  project_id               = var.stackit_project_id
  security_group_id        = stackit_security_group.backend.security_group_id
  direction                = "ingress"
  description              = "Backend HTTPS ports, reachable from the jump host"
  protocol                 = { name = "tcp" }
  port_range               = { min = local.backend_port, max = local.untrusted_port }
  remote_security_group_id = stackit_security_group.jumphost.security_group_id
}
