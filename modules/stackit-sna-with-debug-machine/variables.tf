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

variable "stackit_org_id" {
  description = "The STACKIT Organization ID (UUID)."
  type        = string
  validation {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", var.stackit_org_id))
    error_message = "The stackit_org_id must be a valid UUID."
  }
}

variable "stackit_project_name" {
  description = "The name of the STACKIT project where the managed VPN and test machine will be deployed."
  type        = string
  validation {
    condition     = length(var.stackit_project_name) >= 1 && length(var.stackit_project_name) <= 63
    error_message = "The project name must be between 1 and 63 characters long."
  }
}

variable "stackit_admin_email" {
  description = "The email address of the project administrator."
  type        = string
  validation {
    condition     = can(regex("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", var.stackit_admin_email))
    error_message = "The stackit_admin_email must be a valid email address."
  }
}

variable "sna_name" {
  description = "The name of the STACKIT Network Area (SNA)."
  type        = string
}

variable "sna_transfer_range" {
  description = "The STACKIT SNA transfer range in CIDR notation."
  type        = string
  default     = "172.16.0.0/16"
  validation {
    condition     = can(cidrnetmask(var.sna_transfer_range))
    error_message = "The sna_transfer_range must be a valid CIDR notation."
  }
}

variable "sna_network_range_prefix" {
  description = "A list of STACKIT SNA network range prefixes in CIDR notation."
  type        = list(string)
  default     = ["10.28.0.0/16"]
  validation {
    condition     = alltrue([for r in var.sna_network_range_prefix : can(cidrnetmask(r))])
    error_message = "All elements in sna_network_range_prefix must be valid CIDR notations."
  }
}

variable "sna_default_nameserver" {
  description = "A list of STACKIT SNA default nameservers (IP addresses)."
  type        = list(string)
  default     = ["1.1.1.1"]
  validation {
    condition     = alltrue([for ns in var.sna_default_nameserver : can(regex("^((25[0-5]|(2[0-4]|1\\d|[1-9]|)\\d)\\.?\\b){4}$", ns))])
    error_message = "All elements in sna_default_nameserver must be valid IP addresses."
  }
}

variable "machine_network_name" {
  description = "The name of the network where the test machine will be connected."
  type        = string
}

variable "machine_ipv4_prefix" {
  description = "The IPv4 prefix for the test machine's network (CIDR notation). This must be a subnet within the defined SNA network ranges."
  type        = string
  validation {
    condition     = can(cidrnetmask(var.machine_ipv4_prefix))
    error_message = "The machine_ipv4_prefix must be a valid CIDR notation."
  }
}

variable "machine_name" {
  type        = string
  description = "name of the stackit test machine"
}

variable "machine_availability_zone" {
  description = "The availability zone (e.g. eu01-1)"
  type        = string

  validation {
    condition     = can(regex("^[a-z]{2}[0-9]{2}-[a-zA-Z0-9]+$", var.machine_availability_zone))
    error_message = "The availability zone must follow the STACKIT pattern (e.g., eu01-1, eu01-m)."
  }
}

variable "machine_type" {
  description = "Flavor of the machine"
  type        = string
  default     = "c2i.1"
}

variable "machine_image_id" {
  description = "Image UUID (Default: Debian 12)"
  type        = string
  default     = "c751cde7-e648-4f81-9722-ce9c7848bed0"

  validation {
    condition     = can(regex("^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$", var.machine_image_id))
    error_message = "The image_id must be a valid UUID."
  }
}

variable "machine_disk_size" {
  description = "Boot volume size in GB"
  type        = number
  default     = 20

  validation {
    condition     = var.machine_disk_size >= 1
    error_message = "The disk_size must be at least 1 GB."
  }
}

variable "machine_disk_performance_class" {
  description = "Storage performance class"
  type        = string
  default     = "storage_premium_perf4"
}
