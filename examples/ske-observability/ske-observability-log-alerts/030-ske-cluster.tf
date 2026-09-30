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

data "stackit_ske_kubernetes_versions" "this" {
  version_state = "SUPPORTED"
}

data "stackit_ske_machine_image_versions" "this" {
  version_state = "SUPPORTED"
}

locals {
  flatcar_supported_versions = flatten([
    for mi in data.stackit_ske_machine_image_versions.this.machine_images : [
      for v in mi.versions : v.version if mi.name == "flatcar"
    ]
  ])
  flatcar_supported_version = length(local.flatcar_supported_versions) > 0 ? local.flatcar_supported_versions[0] : null
}

resource "stackit_network" "ske_nodes" {
  project_id  = var.stackit_project_id
  name        = "${var.cluster_name}-nodes"
  ipv4_prefix = var.network_ipv4_prefix
  routed      = true
}

resource "stackit_ske_cluster" "example" {
  project_id             = var.stackit_project_id
  name                   = var.cluster_name
  kubernetes_version_min = data.stackit_ske_kubernetes_versions.this.kubernetes_versions[0].version

  network = {
    id = stackit_network.ske_nodes.network_id
  }

  node_pools = [
    {
      name               = "standard"
      machine_type       = var.node_pool_machine_type
      minimum            = var.node_pool_minimum
      maximum            = var.node_pool_maximum
      max_surge          = 3
      availability_zones = var.node_pool_availability_zones
      os_version_min     = local.flatcar_supported_version
      os_name            = "flatcar"
      volume_size        = 32
      volume_type        = "storage_premium_perf6"
    }
  ]
}

resource "stackit_ske_kubeconfig" "example" {
  project_id   = var.stackit_project_id
  cluster_name = stackit_ske_cluster.example.name
  refresh      = true
}
