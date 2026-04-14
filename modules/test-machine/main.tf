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

terraform {
  required_providers {
    stackit = {
      source = "stackitcloud/stackit"
    }
  }
}

resource "stackit_volume" "boot_volume" {
  project_id        = var.project_id
  name              = "${var.name}-volume"
  availability_zone = var.availability_zone
  size              = var.disk_size
  performance_class = var.disk_performance_class
  source = {
    type = "image"
    id   = var.image_id
  }
}

resource "stackit_network_interface" "nic" {
  project_id = var.project_id
  network_id = var.network_id
  security   = var.security_enabled
}

resource "stackit_server" "server" {
  project_id        = var.project_id
  name              = var.name
  availability_zone = var.availability_zone
  machine_type      = var.machine_type

  boot_volume = {
    source_type = "volume"
    source_id   = stackit_volume.boot_volume.volume_id
  }

  network_interfaces = [
    stackit_network_interface.nic.network_interface_id
  ]

  user_data = var.user_data
}
