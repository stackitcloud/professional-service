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

resource "random_password" "k3s_token" {
  length  = 64
  special = false
}

resource "stackit_key_pair" "cluster" {
  name       = "${var.cluster_name}-keypair"
  public_key = chomp(file(var.ssh_public_key_path))
  labels     = local.common_labels
}

data "stackit_image_v2" "ubuntu" {
  project_id = var.stackit_project_id
  name       = "Ubuntu 22.04"
}

# Public IP on CP1 only — the single external entry point for kubectl and SSH.
# The other 5 nodes communicate via private IP within the cluster network.
resource "stackit_public_ip" "cp1" {
  project_id           = var.stackit_project_id
  network_interface_id = stackit_network_interface.cp[0].network_interface_id
  labels               = local.common_labels
}

locals {
  common_labels = {
    "managed-by" = "terraform"
    "cluster"    = var.cluster_name
  }

  # Cloud-init for the first control plane: initialises the k3s cluster and installs Cilium.
  cp1_user_data = templatefile("${path.module}/cloud-init/cp-init.yaml.tftpl", {
    k3s_version    = var.k3s_version
    k3s_token      = random_password.k3s_token.result
    cp1_public_ip  = stackit_public_ip.cp1.ip
    cp1_private_ip = stackit_network_interface.cp[0].ipv4
    cluster_name   = var.cluster_name
  })

  # Cloud-init for CP2 and CP3: joins the running cluster on CP1.
  cp_join_user_data = templatefile("${path.module}/cloud-init/cp-join.yaml.tftpl", {
    k3s_version    = var.k3s_version
    k3s_token      = random_password.k3s_token.result
    cp1_private_ip = stackit_network_interface.cp[0].ipv4
    cluster_name   = var.cluster_name
  })

  # Cloud-init for all worker nodes: registers as a k3s agent.
  worker_user_data = templatefile("${path.module}/cloud-init/worker.yaml.tftpl", {
    k3s_version    = var.k3s_version
    k3s_token      = random_password.k3s_token.result
    cp1_private_ip = stackit_network_interface.cp[0].ipv4
    cluster_name   = var.cluster_name
  })
}

# --- Control Plane Nodes ---
resource "stackit_server" "cp" {
  count      = 3
  project_id = var.stackit_project_id
  name       = local.cp_names[count.index]

  boot_volume = {
    size                  = var.disk_size_gb
    source_type           = "image"
    source_id             = data.stackit_image_v2.ubuntu.image_id
    delete_on_termination = true
  }

  machine_type       = var.cp_machine_type
  availability_zone  = local.availability_zones[count.index]
  keypair_name       = stackit_key_pair.cluster.name
  network_interfaces = [stackit_network_interface.cp[count.index].network_interface_id]
  labels             = merge(local.common_labels, { "node-role" = "control-plane" })

  # CP1 runs cluster-init + Cilium install; CP2/CP3 join the existing cluster.
  user_data = count.index == 0 ? local.cp1_user_data : local.cp_join_user_data

  depends_on = [
    stackit_public_ip.cp1,
  ]
}

# --- Worker Nodes ---
resource "stackit_server" "worker" {
  count      = 3
  project_id = var.stackit_project_id
  name       = local.worker_names[count.index]

  boot_volume = {
    size                  = var.disk_size_gb
    source_type           = "image"
    source_id             = data.stackit_image_v2.ubuntu.image_id
    delete_on_termination = true
  }

  machine_type       = var.worker_machine_type
  availability_zone  = local.availability_zones[count.index]
  keypair_name       = stackit_key_pair.cluster.name
  network_interfaces = [stackit_network_interface.worker[count.index].network_interface_id]
  user_data          = local.worker_user_data
  labels             = merge(local.common_labels, { "node-role" = "worker" })
}
