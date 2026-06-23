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

resource "stackit_server" "example01" {
  project_id = var.stackit_project_id
  name       = "example01"
  boot_volume = {
    size                  = 64
    source_type           = "image"
    source_id             = var.debian_image_id
    performance_class     = "storage_premium_perf6"
    delete_on_termination = true
  }
  machine_type       = "c2i.4"
  availability_zone  = "eu01-1"
  user_data          = local.user_data_master
  keypair_name       = stackit_key_pair.admin_keypair.name
  network_interfaces = [stackit_network_interface.example01.network_interface_id]
}

resource "stackit_network_interface" "example01" {
  project_id = var.stackit_project_id
  network_id = stackit_network.default.network_id
  # security   = false
  allowed_addresses  = [format("%s/%s", stackit_network_interface.vip01.ipv4, "32")]
  security_group_ids = [stackit_security_group.active-passive.security_group_id]
}

resource "stackit_public_ip" "example01-wan" {
  project_id           = var.stackit_project_id
  network_interface_id = stackit_network_interface.example01.network_interface_id
}
