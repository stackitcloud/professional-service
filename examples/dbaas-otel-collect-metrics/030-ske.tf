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

resource "stackit_ske_kubeconfig" "this" {
  project_id   = var.stackit_project_id
  cluster_name = stackit_ske_cluster.this.name
  refresh      = true

  depends_on = [stackit_ske_cluster.this]
}

data "stackit_ske_kubernetes_versions" "this" {
  version_state = "SUPPORTED"
}

data "stackit_ske_machine_image_versions" "this" {
  version_state = "SUPPORTED"
}

locals {
  flatcar_supported_version = one(flatten([
    for mi in data.stackit_ske_machine_image_versions.this.machine_images : [
      for v in mi.versions :
      v.version
      if mi.name == "flatcar"
    ]
  ]))
}

resource "stackit_ske_cluster" "this" {
  project_id             = var.stackit_project_id
  name                   = "dbaas-otel"
  kubernetes_version_min = data.stackit_ske_kubernetes_versions.this.kubernetes_versions.0.version

  maintenance = {
    enable_kubernetes_version_updates    = true
    enable_machine_image_version_updates = true
    start                                = "01:00:00Z"
    end                                  = "02:00:00Z"
  }

  node_pools = [
    {
      name               = "standard"
      machine_type       = "g2i.4"
      minimum            = "3"
      maximum            = "9"
      max_surge          = "3"
      availability_zones = ["eu01-1", "eu01-2", "eu01-3"]
      os_version_min     = local.flatcar_supported_version
      os_name            = "flatcar"
      volume_size        = 150
      volume_type        = "storage_premium_perf6"
    },
  ]
}
