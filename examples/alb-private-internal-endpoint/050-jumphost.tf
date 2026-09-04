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

data "stackit_image_v2" "debian" {
  project_id = var.stackit_project_id
  name       = var.image_name
}

# The jump host is the only resource with a public address. It exists so that
# the private load balancer can be tested without a VPN; in production the
# clients come from on-premises or from other projects instead.
resource "stackit_key_pair" "jumphost" {
  name       = "${var.name_prefix}-jumphost"
  public_key = chomp(var.ssh_public_key)
  labels     = local.labels
}

resource "stackit_network_interface" "jumphost" {
  project_id         = var.stackit_project_id
  network_id         = stackit_network.listener.network_id
  name               = "${var.name_prefix}-jumphost"
  security           = true
  security_group_ids = [stackit_security_group.jumphost.security_group_id]
}

resource "stackit_public_ip" "jumphost" {
  project_id           = var.stackit_project_id
  network_interface_id = stackit_network_interface.jumphost.network_interface_id
  labels               = local.labels
}

resource "stackit_server" "jumphost" {
  project_id        = var.stackit_project_id
  name              = "${var.name_prefix}-jumphost"
  availability_zone = var.availability_zones[0]
  machine_type      = var.machine_type
  keypair_name      = stackit_key_pair.jumphost.name
  labels            = local.labels

  boot_volume = {
    source_type           = "image"
    source_id             = data.stackit_image_v2.debian.image_id
    size                  = var.boot_volume_size_gb
    delete_on_termination = true
  }

  network_interfaces = [stackit_network_interface.jumphost.network_interface_id]

  # The jump host trusts the private CA, so curl and openssl can verify the
  # certificates of the load balancer and the backends without extra options.
  user_data = templatefile("${path.module}/jumphost-cloud-init.yaml.tftpl", {
    ca_cert = tls_self_signed_cert.ca.cert_pem
  })
}
