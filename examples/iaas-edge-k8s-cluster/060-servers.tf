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

# Talos ignores cloud-init — no user_data or key pair needed.
# Do NOT clone these VMs: Talos uses the disk UUID as its EdgeHost identity.

resource "stackit_server" "cp" {
  count             = var.cp_count
  project_id        = var.stackit_project_id
  name              = "edge-cluster-cp-${count.index}"
  machine_type      = var.cp_machine_type
  availability_zone = var.availability_zone

  boot_volume = {
    size        = var.disk_size_gb
    source_type = "image"
    source_id   = data.external.image_ids.result.image_id
  }

  network_interfaces = [stackit_network_interface.cp[count.index].network_interface_id]
}

resource "stackit_server" "worker" {
  count             = var.worker_count
  project_id        = var.stackit_project_id
  name              = "edge-cluster-worker-${count.index}"
  machine_type      = var.worker_machine_type
  availability_zone = var.availability_zone

  boot_volume = {
    size        = var.disk_size_gb
    source_type = "image"
    source_id   = data.external.image_ids.result.image_id
  }

  network_interfaces = [stackit_network_interface.worker[count.index].network_interface_id]
}
