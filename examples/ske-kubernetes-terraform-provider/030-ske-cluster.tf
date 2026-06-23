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

resource "stackit_ske_cluster" "example" {
  project_id             = var.project_id
  name                   = "example"
  kubernetes_version_min = "1.33"

  node_pools = [
    {
      name               = "example-node-pool"
      machine_type       = "g2i.4"
      minimum            = 1
      maximum            = 2
      availability_zones = ["eu01-1"]
      os_version_min     = "3815.2.5"
      os_name            = "flatcar"
      volume_size        = 32
      volume_type        = "storage_premium_perf6"
    }
  ]
}

resource "stackit_ske_kubeconfig" "example" {
  project_id   = var.project_id
  cluster_name = stackit_ske_cluster.example.name
  expiration   = 3600
}
