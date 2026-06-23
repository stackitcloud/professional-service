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

resource "stackit_server" "encrypted_server" {
  project_id = var.STACKIT_PROJECT_ID
  name       = "encrypted-server"
  boot_volume = {
    source_type = "volume"
    source_id   = stackit_volume.encrypted.volume_id
  }
  availability_zone  = var.zone
  machine_type       = "g2i.4"
  user_data          = file("cloud-init.yaml")
  network_interfaces = [stackit_network_interface.nic.network_interface_id]
}

resource "stackit_network_interface" "nic" {
  project_id         = var.STACKIT_PROJECT_ID
  network_id         = data.stackit_network.default.network_id
  security_group_ids = [data.stackit_security_group.default.security_group_id]
}

data "stackit_security_group" "default" {
  project_id        = var.STACKIT_PROJECT_ID
  security_group_id = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}

data "stackit_network" "default" {
  project_id = var.STACKIT_PROJECT_ID
  network_id = "a9d59cc6-cc5b-4f9f-a9dc-315b0fc78a35"
}
