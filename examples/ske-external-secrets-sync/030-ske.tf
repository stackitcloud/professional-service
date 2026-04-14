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

resource "stackit_ske_cluster" "ske_cluster_01" {
  project_id             = var.stackit_project_id
  name                   = "secret-test"
  kubernetes_version_min = "1.33"

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
      os_version_min     = "4230.2.1"
      os_name            = "flatcar"
      volume_size        = 32
      volume_type        = "storage_premium_perf6"
    },
  ]
}

resource "stackit_ske_kubeconfig" "ske_kubeconfig_01" {
  project_id   = var.stackit_project_id
  cluster_name = stackit_ske_cluster.ske_cluster_01.name
  refresh      = true

  depends_on = [stackit_ske_cluster.ske_cluster_01]
}
