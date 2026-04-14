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
  description = "The STACKIT Project ID"
  type        = string
}

variable "network_id" {
  description = "The Network ID (UUID) where the machine should be spawned"
  type        = string
}

variable "name" {
  description = "Hostname of the server"
  type        = string
  default     = "test-machine"
}




variable "availability_zone" {
  description = "The availability zone (e.g. eu01-1)"
  type        = string
}

variable "machine_type" {
  description = "Flavor of the machine"
  type        = string
  default     = "g1.1"
}

variable "image_id" {
  description = "Image UUID (Default: Debian 12)"
  type        = string
  default     = "c751cde7-e648-4f81-9722-ce9c7848bed0"
}

variable "disk_size" {
  description = "Boot volume size in GB"
  type        = number
  default     = 50
}

variable "disk_performance_class" {
  description = "Storage performance class"
  type        = string
  default     = "storage_premium_perf4"
}

variable "user_data" {
  description = "Cloud-init configuration string"
  type        = string
  default     = ""
}

variable "security_enabled" {
  description = "Enable security (port security) on the network interface"
  type        = bool
  default     = false
}
