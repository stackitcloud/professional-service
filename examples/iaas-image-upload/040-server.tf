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

# Example: boot a server from the uploaded custom image.
# The image_id is referenced directly from the stackit_image resource above.

resource "stackit_network" "main" {
  project_id         = var.project_id
  name               = "${var.server_name}-network"
  ipv4_prefix_length = tonumber(split("/", var.network_cidr)[1])
}

resource "stackit_network_interface" "server" {
  project_id = var.project_id
  network_id = stackit_network.main.network_id
  name       = "${var.server_name}-nic"
}

resource "stackit_key_pair" "server" {
  name       = var.keypair_name
  public_key = chomp(var.ssh_public_key)
}

resource "stackit_server" "from_custom_image" {
  project_id        = var.project_id
  name              = var.server_name
  machine_type      = var.machine_type
  availability_zone = var.availability_zone
  keypair_name      = stackit_key_pair.server.name

  boot_volume = {
    source_type = "image"
    source_id   = stackit_image.custom_image.image_id
    size        = var.boot_volume_size_gb
  }

  network_interfaces = [stackit_network_interface.server.network_interface_id]

  labels = var.labels
}
