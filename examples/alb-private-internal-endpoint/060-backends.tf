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

# With disable_target_security_group_assignment the load balancer does not
# touch the backend interfaces. Its target security group, which permits the
# traffic from the load balancer, is assigned here together with the group of
# the example. The interfaces therefore depend on the load balancer and use
# the fixed addresses that the load balancer already knows as targets.
resource "stackit_network_interface" "backend" {
  for_each = local.backends

  project_id = var.stackit_project_id
  network_id = stackit_network.target.network_id
  name       = local.backend_names[each.key]
  ipv4       = local.backend_ips[each.key]
  security   = true

  security_group_ids = [
    stackit_security_group.backend.security_group_id,
    stackit_application_load_balancer.this.target_security_group.id,
  ]
}

resource "stackit_server" "backend" {
  for_each = local.backends

  project_id        = var.stackit_project_id
  name              = local.backend_names[each.key]
  availability_zone = each.value
  machine_type      = var.machine_type
  labels            = local.labels

  boot_volume = {
    source_type           = "image"
    source_id             = data.stackit_image_v2.debian.image_id
    size                  = var.boot_volume_size_gb
    delete_on_termination = true
  }

  network_interfaces = [stackit_network_interface.backend[each.key].network_interface_id]

  # Each backend receives its own certificate and key, issued by the private CA
  # at apply time. The key travels in the user data and is stored in the state.
  user_data = templatefile("${path.module}/backend-cloud-init.yaml.tftpl", {
    server_py      = file("${path.module}/files/server.py")
    server_cert    = tls_locally_signed_cert.backend[each.key].cert_pem
    server_key     = tls_private_key.backend[each.key].private_key_pem
    backend_port   = local.backend_port
    untrusted_port = local.untrusted_port
  })
}
