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
  project_id             = stackit_resourcemanager_project.this.project_id
  name                   = var.cluster_name
  kubernetes_version_min = var.kubernetes_version_min

  node_pools = [
    {
      name               = "default"
      machine_type       = var.node_machine_type
      os_name            = var.node_os_name
      minimum            = var.node_min
      maximum            = var.node_max
      availability_zones = [var.availability_zone]
      volume_type        = "storage_premium_perf1"
      volume_size        = "32"
    }
  ]

  maintenance = {
    enable_kubernetes_version_updates    = true
    enable_machine_image_version_updates = true
    start                                = "01:00:00Z"
    end                                  = "02:00:00Z"
  }

  network = {
    control_plane = {
      access_scope = "PUBLIC"
    }
  }
}

resource "stackit_ske_kubeconfig" "this" {
  project_id   = stackit_resourcemanager_project.this.project_id
  cluster_name = stackit_ske_cluster.this.name
  refresh      = true
}

resource "local_sensitive_file" "kubeconfig" {
  content         = stackit_ske_kubeconfig.this.kube_config
  filename        = "${path.module}/../.kubeconfig"
  file_permission = "0600"
}
