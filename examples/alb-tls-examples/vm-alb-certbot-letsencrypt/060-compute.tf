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

locals {
  cloud_init_rendered = templatefile("${path.root}/templates/cloud-init.yaml.tpl", {
    start_nginx_test_container = var.start_nginx_test_container
  })
}

resource "stackit_key_pair" "workshop" {
  name       = var.keypair_name
  public_key = chomp(var.ssh_public_key)

  labels = {
    environment = "workshop"
    managed-by  = "terraform"
  }
}

resource "stackit_network_interface" "vm" {
  project_id         = stackit_resourcemanager_project.showcase.project_id
  network_id         = stackit_network.workshop.network_id
  name               = "${var.vm_name}-nic"
  security_group_ids = [stackit_security_group.vm.security_group_id]

  # The ALB injects its target_security_group into this NIC after creation —
  # ignore_changes prevents Terraform from reverting it on subsequent applies
  lifecycle {
    ignore_changes = [security_group_ids]
  }
}

resource "stackit_public_ip" "vm" {
  project_id           = stackit_resourcemanager_project.showcase.project_id
  network_interface_id = stackit_network_interface.vm.network_interface_id

  labels = {
    environment = "workshop"
    managed-by  = "terraform"
  }
}

resource "stackit_server" "docker_host" {
  project_id        = stackit_resourcemanager_project.showcase.project_id
  name              = var.vm_name
  machine_type      = var.machine_type
  availability_zone = var.availability_zone
  keypair_name      = stackit_key_pair.workshop.name
  user_data         = local.cloud_init_rendered

  boot_volume = {
    source_type = "image"
    source_id   = var.image_id
    size        = var.boot_volume_size_gb
  }

  network_interfaces = [stackit_network_interface.vm.network_interface_id]

  agent = {
    provisioning_policy = "ALWAYS"
  }

  labels = {
    environment = "workshop"
    managed-by  = "terraform"
    role        = "docker-host"
    use-case    = "alb-certbot"
  }

  depends_on = [
    stackit_network_interface.vm,
    stackit_key_pair.workshop,
  ]
}
