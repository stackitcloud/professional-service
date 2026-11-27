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


// Palo Alto firewalls, one per availability zone.
// The interface order defines the PA interface mapping: mgmt, ethernet1/1 (HA), ethernet1/2 (WAN)
resource "stackit_server" "fw" {
  for_each          = var.firewalls
  project_id        = local.project_id
  name              = each.key
  availability_zone = each.value.availability_zone
  machine_type      = var.flavor
  boot_volume = {
    source_type           = "image"
    source_id             = stackit_image.paloalto.image_id
    size                  = var.boot_volume_size
    performance_class     = var.boot_volume_performance_class
    delete_on_termination = true
  }
  network_interfaces = [
    stackit_network_interface.mgmt[each.key].network_interface_id,
    stackit_network_interface.ha[each.key].network_interface_id,
    stackit_network_interface.wan[each.key].network_interface_id,
  ]
}

// LAN interfaces are attached one after another to keep the order: ethernet1/3 (LAN1), ethernet1/4 (LAN2)
resource "stackit_server_network_interface_attach" "lan1" {
  for_each             = var.firewalls
  project_id           = local.project_id
  server_id            = stackit_server.fw[each.key].server_id
  network_interface_id = stackit_network_interface.lan1[each.key].network_interface_id
}

resource "stackit_server_network_interface_attach" "lan2" {
  for_each             = var.firewalls
  project_id           = local.project_id
  server_id            = stackit_server.fw[each.key].server_id
  network_interface_id = stackit_network_interface.lan2[each.key].network_interface_id

  depends_on = [stackit_server_network_interface_attach.lan1]
}
