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

data "stackit_image_v2" "backend" {
  project_id = var.stackit_project_id
  name       = var.image_name
}

resource "stackit_network_interface" "backend" {
  for_each = local.backends

  project_id = var.stackit_project_id
  network_id = stackit_network.this.network_id
  name       = "${var.name_prefix}-backend-${each.key}"
  security   = true

  security_group_ids = [stackit_security_group.backend.security_group_id]

  # The load balancer adds its own target security group to the interface.
  lifecycle {
    ignore_changes = [security_group_ids]
  }
}

resource "stackit_server" "backend" {
  for_each = local.backends

  project_id        = var.stackit_project_id
  name              = "${var.name_prefix}-backend-${each.key}"
  availability_zone = each.value
  machine_type      = var.machine_type
  labels            = local.labels

  boot_volume = {
    source_type           = "image"
    source_id             = data.stackit_image_v2.backend.image_id
    size                  = var.boot_volume_size_gb
    delete_on_termination = true
  }

  network_interfaces = [stackit_network_interface.backend[each.key].network_interface_id]

  user_data = templatefile("${path.module}/cloud-init.yaml.tftpl", {
    server_py         = file("${path.module}/files/server.py")
    backend_arguments = local.backend_arguments
  })
}
