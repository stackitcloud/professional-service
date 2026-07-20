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

variable "project_id" {
  description = "The STACKIT project ID"
  type        = string
}

variable "cluster_name" {
  description = "The name of the Kubernetes cluster"
  type        = string
  default     = null
}

variable "network_id" {
  description = "The ID of the STACKIT network in which the SKE cluster will be deployed. If not provided, the cluster will automatically create a network on demand. Specifying a network ID is only supported in SNA setups"
  type        = string
  default     = null
}

variable "maintenance" {
  description = "Maintenance window configuration for the cluster"
  type = object({
    enable_kubernetes_version_updates    = bool
    enable_machine_image_version_updates = bool
    start                                = string
    end                                  = string
  })
  default = {
    enable_kubernetes_version_updates    = true
    enable_machine_image_version_updates = true
    start                                = "04:00:00Z"
    end                                  = "05:00:00Z"
  }
}

variable "node_pools" {
  description = "Configuration for the cluster node pools"
  type        = any
  default = [
    {
      name               = "standard"
      machine_type       = "g2i.4"
      minimum            = 1
      maximum            = 3
      max_surge          = 3
      availability_zones = ["eu01-1", "eu01-2", "eu01-3"]
      os_name            = "flatcar"
      volume_size        = 20
      volume_type        = "storage_premium_perf6"
    }
  ]
}
