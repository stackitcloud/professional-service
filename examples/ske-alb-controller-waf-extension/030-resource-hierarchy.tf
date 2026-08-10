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
}

resource "stackit_network_area_region" "this" {
  organization_id = var.stackit_org_id
  network_area_id = stackit_network_area.this.network_area_id
  ipv4 = {
    transfer_network = var.sna_transfer_range
    network_ranges = [
      {
        prefix = var.sna_network_range
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

resource "stackit_network" "ske_nodes" {
  name             = "ske-alb-nodes"
  project_id       = stackit_resourcemanager_project.this.project_id
  ipv4_prefix      = var.network_ipv4_prefix
  ipv4_nameservers = var.sna_default_nameserver

  depends_on = [stackit_network_area_region.this]
}
