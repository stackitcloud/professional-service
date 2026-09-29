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

# The cluster ID condition defers the kubeconfig request until the cluster exists.
ephemeral "stackit_ske_kubeconfig" "this" {
  project_id   = var.stackit_project_id
  cluster_name = stackit_ske_cluster.this.id != "" ? stackit_ske_cluster.this.name : ""
  # Two hours cover a first apply in which the database finishes long after the cluster.
  expiration = 7200
}

resource "stackit_ske_cluster" "this" {
  project_id = var.stackit_project_id
  name       = "dbaas-otel"

  node_pools = [
    {
      name               = "standard"
      machine_type       = "g2i.4"
      minimum            = "3"
      maximum            = "9"
      max_surge          = "3"
      availability_zones = [for z in [1, 2, 3] : "${var.stackit_region}-${z}"]
      os_name            = "flatcar"
      volume_size        = 150
      volume_type        = "storage_premium_perf6"
    },
  ]
}
