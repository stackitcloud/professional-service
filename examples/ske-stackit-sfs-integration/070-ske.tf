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

resource "stackit_ske_cluster" "sfs" {
  project_id             = stackit_resourcemanager_project.sfs-no-folder.project_id
  name                   = "sfs"
  kubernetes_version_min = "1.34"
  node_pools = [
    {
      name               = "np-example"
      machine_type       = "c2i.2"
      minimum            = "1"
      maximum            = "3"
      availability_zones = ["eu01-3"]
    }
  ]
  network = {
    id = stackit_network.sfs-example.network_id
  }
  maintenance = {
    enable_kubernetes_version_updates    = true
    enable_machine_image_version_updates = true
    start                                = "01:00:00Z"
    end                                  = "02:00:00Z"
  }
}

resource "stackit_network" "sfs-example" {
  project_id       = stackit_resourcemanager_project.sfs-no-folder.project_id
  name             = "ske-example"
  ipv4_nameservers = ["9.9.9.9"]
}
