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
  ipv4_prefix      = "10.0.1.0/24"
  ipv4_nameservers = ["9.9.9.9", "1.1.1.1"]
  labels           = local.common_labels
}

resource "stackit_security_group" "cluster" {
  project_id = var.stackit_project_id
  name       = "${var.cluster_name}-sg"
  labels     = local.common_labels
}

# SSH access for Terraform bootstrap provisioner
resource "stackit_security_group_rule" "ssh" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 22
    max = 22
  }
  protocol = {
    name = "tcp"
  }
}

# Kubernetes API server — needed for external kubectl and ALB health checks
resource "stackit_security_group_rule" "k8s_api" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 6443
    max = 6443
  }
  protocol = {
    name = "tcp"
  }
}

# HTTP/HTTPS — required for ALB controller test ingress
resource "stackit_security_group_rule" "http" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 80
    max = 80
  }
  protocol = {
    name = "tcp"
  }
}

resource "stackit_security_group_rule" "https" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 443
    max = 443
  }
  protocol = {
    name = "tcp"
  }
}

# NodePort range — NLB targets need to reach these ports
resource "stackit_security_group_rule" "nodeport" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 30000
    max = 32767
  }
  protocol = {
    name = "tcp"
  }
}

# etcd peer communication between control planes
resource "stackit_security_group_rule" "etcd" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 2379
    max = 2380
  }
  protocol = {
    name = "tcp"
  }
}

# kubelet API — required by CCM/CSI for node introspection
resource "stackit_security_group_rule" "kubelet" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 10250
    max = 10250
  }
  protocol = {
    name = "tcp"
  }
}

# Cilium VXLAN tunnel (UDP)
resource "stackit_security_group_rule" "cilium_vxlan" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 8472
    max = 8472
  }
  protocol = {
    name = "udp"
  }
}

# Cilium health checks
resource "stackit_security_group_rule" "cilium_health" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  port_range = {
    min = 4240
    max = 4240
  }
  protocol = {
    name = "tcp"
  }
}

# Allow all intra-cluster TCP and UDP (node-to-node on any port).
# The STACKIT provider does not support source IP filtering on security group rules,
# so these rules allow from any source — acceptable for a private cluster network.
resource "stackit_security_group_rule" "intra_cluster" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  protocol = {
    name = "tcp"
  }
}

resource "stackit_security_group_rule" "intra_cluster_udp" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "ingress"
  protocol = {
    name = "udp"
  }
}

# Allow all egress (nodes need to reach GitHub, container registries, STACKIT APIs)
resource "stackit_security_group_rule" "egress_all" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "egress"
  protocol = {
    name = "tcp"
  }
}

resource "stackit_security_group_rule" "egress_all_udp" {
  project_id        = var.stackit_project_id
  security_group_id = stackit_security_group.cluster.security_group_id
  direction         = "egress"
  protocol = {
    name = "udp"
  }
}

# --- Network interfaces (pre-created to resolve private IPs before server creation)
# Terraform creates these resources first, making .ipv4 available as a known value.
# This allows us to embed CP1's private IP into the cloud-init of subsequent nodes.
resource "stackit_network_interface" "cp" {
  count              = 3
  project_id         = var.stackit_project_id
  network_id         = stackit_network.cluster.network_id
  security_group_ids = [stackit_security_group.cluster.security_group_id]
}

resource "stackit_network_interface" "worker" {
  count              = 3
  project_id         = var.stackit_project_id
  network_id         = stackit_network.cluster.network_id
  security_group_ids = [stackit_security_group.cluster.security_group_id]
}
