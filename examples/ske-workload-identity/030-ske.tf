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

resource "stackit_ske_cluster" "this" {
  project_id             = var.stackit_project_id
  name                   = var.cluster_name
  kubernetes_version_min = "1.35.6"

  node_pools = [
    {
      name               = "default"
      machine_type       = "g2i.4"
      minimum            = "1"
      maximum            = "3"
      max_surge          = "1"
      availability_zones = ["eu01-1"]
      os_name            = "flatcar"
      volume_size        = 50
      volume_type        = "storage_premium_perf4"
    }
  ]
}

resource "stackit_ske_kubeconfig" "this" {
  project_id   = var.stackit_project_id
  cluster_name = stackit_ske_cluster.this.name
  refresh      = true

  depends_on = [stackit_ske_cluster.this]
}
