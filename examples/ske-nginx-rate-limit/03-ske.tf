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

resource "random_string" "random" {
  length  = 4
  lower   = true
  upper   = false
  special = false
}

resource "stackit_ske_cluster" "default" {
  project_id             = var.stackit_project_id
  name                   = "ske-${random_string.random.result}"
  kubernetes_version_min = "1.31"
  node_pools = [
    {
      name               = "standard"
      machine_type       = "c1.4"
      minimum            = "1"
      maximum            = "3"
      max_surge          = "3"
      availability_zones = ["eu01-1", "eu01-2", "eu01-3"]
      os_version_min     = "4152.2.1"
      os_name            = "flatcar"
      volume_size        = 32
      volume_type        = "storage_premium_perf6"
    }
  ]
  maintenance = {
    enable_kubernetes_version_updates    = true
    enable_machine_image_version_updates = true
    start                                = "01:00:00Z"
    end                                  = "02:00:00Z"
  }
}

resource "stackit_ske_kubeconfig" "example" {
  project_id   = var.stackit_project_id
  cluster_name = stackit_ske_cluster.default.name
  refresh      = true
}
