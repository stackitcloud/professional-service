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

#
# Custom User Settings
#

# Only the network area needs this. Projects go to stackit_parent_container_id.
variable "stackit_org_id" {
  type        = string
  description = "STACKIT organization ID the network area belongs to"
}

variable "stackit_parent_container_id" {
  type        = string
  description = "Container the project is created in. The organization, because SKE rejects a cluster in a folder-nested project."
}

variable "stackit_admin_email" {
  type        = string
  description = "Email address that becomes the owner of the created projects"
}

variable "stackit_region" {
  type        = string
  description = "STACKIT region"
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the service account key file. Unset falls back to the STACKIT_SERVICE_ACCOUNT_KEY_PATH environment variable, then to that entry in the credentials file ($HOME/.stackit/credentials.json, or STACKIT_CREDENTIALS_PATH)."
  default     = null
}

#
# Network area
#

variable "sna_name" {
  type        = string
  description = "Name of the network area"
  default     = "sfs-network-area"
}

variable "sna_transfer_network" {
  type        = string
  description = "Transfer network of the network area"
  default     = "10.1.2.0/24"
}

variable "sna_network_range" {
  type        = string
  description = "Range the network area hands out to its projects"
  default     = "10.0.0.0/16"
}

#
# Projects
#

variable "project_name" {
  type        = string
  description = "Name of the project"
  default     = "sfs-example"
}

#
# STACKIT File Storage
#

variable "sfs_availability_zone" {
  type        = string
  description = "Availability zone of the SFS resource pool"
  default     = "eu01-m"
}

variable "sfs_performance_class" {
  type        = string
  description = "Performance class of the SFS resource pool"
  default     = "Standard"
}

variable "sfs_pool_size_gigabytes" {
  type        = number
  description = "Size of the SFS resource pool in gigabytes. The minimum is 500, the maximum 20 TB."
  default     = 512

  validation {
    condition     = var.sfs_pool_size_gigabytes >= 500
    error_message = "A STACKIT File Storage resource pool needs at least 500 gigabytes."
  }
}

variable "sfs_share_size_gigabytes" {
  type        = number
  description = "Space hard limit of the SFS share in gigabytes"
  default     = 128
}

# Defaults to the network range of the network area, so only workload inside the
# network area can mount the share. Never open this to 0.0.0.0/0.
variable "sfs_ip_acl" {
  type        = list(string)
  description = "Networks allowed to mount the resource pool and the share"
  default     = null
}

# The SDK defaults every pool wait handler to 10 minutes, which is too short.
# A timeout leaves the pool tainted, so the next run replaces one that exists.
# See the README.
variable "sfs_pool_operation_timeout" {
  type        = string
  description = "Timeout for creating, updating and deleting an SFS resource pool"
  default     = "60m"
}

variable "sfs_labels" {
  type        = map(string)
  description = "Labels attached to the SFS resources. Keys start with a lowercase letter, values allow lowercase letters, digits, underscore and hyphen."
  default = {
    example = "ske-stackit-sfs-integration"
  }
}
