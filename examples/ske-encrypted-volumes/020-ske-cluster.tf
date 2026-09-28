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

resource "stackit_ske_cluster" "default" {
  project_id = var.stackit_project_id
  name       = "ske-enc-vol"
  # Unset, the cluster is created with the latest Kubernetes version that
  # SKE supports. Uncomment to pin a minimum version instead. A pinned
  # version stops working once SKE removes it.
  # kubernetes_version_min = "1.36"

  node_pools = [{
    name               = "standard"
    machine_type       = "c2i.4"
    minimum            = 1
    maximum            = 3
    availability_zones = ["eu01-1"]
    os_name            = "flatcar"
    volume_size        = 32
  }]
}

resource "stackit_ske_kubeconfig" "default" {
  project_id   = var.stackit_project_id
  cluster_name = stackit_ske_cluster.default.name
  refresh      = true
}

data "stackit_service_accounts" "ske_internal" {
  project_id   = var.stackit_project_id
  email_suffix = "@ske.sa.stackit.cloud"

  depends_on = [stackit_ske_cluster.default]
}
