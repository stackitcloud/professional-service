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
  ubuntu_supported_version = one(flatten([
    for mi in data.stackit_ske_machine_image_versions.this.machine_images : [
      for v in mi.versions :
      v.version
      if mi.name == "ubuntu"
    ]
  ]))
}

resource "stackit_ske_cluster" "this" {
  project_id             = var.project_id
  name                   = "nested-virt"
  kubernetes_version_min = data.stackit_ske_kubernetes_versions.this.kubernetes_versions.0.version

  maintenance = {
    enable_kubernetes_version_updates    = true
    enable_machine_image_version_updates = true
    start                                = "01:00:00Z"
    end                                  = "02:00:00Z"
  }

  node_pools = [
    # g3i — Intel VMX, nested virt enabled → expect PASS
    {
      name               = "g3-intel"
      machine_type       = "g3i.2"
      minimum            = "1"
      maximum            = "1"
      max_surge          = "1"
      availability_zones = ["eu01-1"]
      os_name            = "ubuntu"
      os_version_min     = local.ubuntu_supported_version
      volume_size        = 20
      volume_type        = "storage_premium_perf6"
      labels = {
        "nested-virt-demo" = "true"
        "pool"             = "g3-intel"
      }
    },

    # g2i — Intel VMX, nested virt enabled → expect PASS
    {
      name               = "g2-intel"
      machine_type       = "g2i.8"
      minimum            = "1"
      maximum            = "1"
      max_surge          = "1"
      availability_zones = ["eu01-1"]
      os_name            = "ubuntu"
      os_version_min     = local.ubuntu_supported_version
      volume_size        = 20
      volume_type        = "storage_premium_perf6"
      labels = {
        "nested-virt-demo" = "true"
        "pool"             = "g2-intel"
      }
    },

    # c2a — AMD, nested virt NOT enabled → expect FAIL (intentional)
    {
      name               = "c2a-amd-no-kvm"
      machine_type       = "c2a.2d"
      minimum            = "1"
      maximum            = "1"
      max_surge          = "1"
      availability_zones = ["eu01-1"]
      os_name            = "ubuntu"
      os_version_min     = local.ubuntu_supported_version
      volume_size        = 20
      volume_type        = "storage_premium_perf6"
      labels = {
        "nested-virt-demo" = "true"
        "pool"             = "c2a-amd-no-kvm"
      }
    },
  ]
}

ephemeral "stackit_ske_kubeconfig" "this" {
  project_id   = var.project_id
  cluster_name = stackit_ske_cluster.this.id != "" ? stackit_ske_cluster.this.name : ""
}
