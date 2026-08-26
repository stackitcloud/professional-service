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

resource "stackit_network_area" "this" {
  name            = var.sna_name
  organization_id = var.stackit_org_id
  labels = {
    "preview/routingtables" = "true"
  }
}

resource "stackit_network_area_region" "this" {
  organization_id = var.stackit_org_id
  network_area_id = stackit_network_area.this.network_area_id
  ipv4 = {
    transfer_network = var.sna_transfer_range
    network_ranges = [
      for prefix in var.sna_network_range_prefix : {
        prefix = prefix
      }
    ]
    default_nameservers = var.sna_default_nameserver
  }
}

resource "stackit_resourcemanager_project" "this" {
  parent_container_id = var.stackit_org_id
  name                = var.stackit_project_name
  owner_email         = var.stackit_admin_email
  labels = {
    "networkArea" = stackit_network_area.this.network_area_id
  }
}

resource "stackit_volume" "this" {
  project_id        = stackit_resourcemanager_project.this.project_id
  name              = "${var.machine_name}-volume"
  availability_zone = var.machine_availability_zone
  size              = var.machine_disk_size
  performance_class = var.machine_disk_performance_class
  source = {
    type = "image"
    id   = var.machine_image_id
  }
}

resource "stackit_network" "this" {
  name             = var.machine_network_name
  project_id       = stackit_resourcemanager_project.this.project_id
  ipv4_prefix      = var.machine_ipv4_prefix
  ipv4_nameservers = var.sna_default_nameserver
}

resource "stackit_network_interface" "this" {
  project_id = stackit_resourcemanager_project.this.project_id
  network_id = stackit_network.this.network_id
  security   = false
}

resource "stackit_server" "this" {
  project_id        = stackit_resourcemanager_project.this.project_id
  name              = var.machine_name
  availability_zone = var.machine_availability_zone
  machine_type      = var.machine_type

  boot_volume = {
    source_type = "volume"
    source_id   = stackit_volume.this.volume_id
  }

  agent = {
    provisioning_policy = "ALWAYS"
  }

  network_interfaces = [
    stackit_network_interface.this.network_interface_id
  ]

  user_data = file("${path.module}/debug-user.yml")
}

resource "stackit_public_ip" "this" {
  project_id           = stackit_resourcemanager_project.this.project_id
  network_interface_id = stackit_network_interface.this.network_interface_id
}
