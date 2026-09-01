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

variable "stackit_project_id" {
  description = "The STACKIT project ID to deploy resources into."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$", var.stackit_project_id))
    error_message = "The stackit_project_id must be a valid UUID."
  }
}

variable "stackit_region" {
  description = "The STACKIT region to deploy resources into."
  type        = string
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  description = "Path to the STACKIT service account key JSON file used for provider authentication."
  type        = string
}

variable "name_prefix" {
  description = "Prefix applied to the names of all resources. Lowercase letters, digits and hyphens only."
  type        = string
  default     = "alb-priv"

  validation {
    condition     = can(regex("^[a-z0-9]+(-[a-z0-9]+)*$", var.name_prefix)) && length(var.name_prefix) <= 20
    error_message = "The name_prefix must be 1-20 characters of lowercase letters, digits and single hyphens, starting and ending with a letter or digit."
  }
}

variable "internal_domain" {
  description = "Domain of the internal API. The listener routes api.<internal_domain>; the backend certificates carry <backend name>.<internal_domain>. No DNS zone is created."
  type        = string
  default     = "internal.example"

  validation {
    condition     = can(regex("^[a-z0-9]+([-.][a-z0-9]+)*$", var.internal_domain))
    error_message = "The internal_domain must consist of lowercase letters, digits, hyphens and dots."
  }
}

variable "listener_network_cidr" {
  description = "IPv4 prefix of the network that carries the listener of the load balancer and the jump host. In a project that belongs to a STACKIT Network Area the prefix must lie inside the network ranges of that area."
  type        = string
  default     = "10.20.2.0/24"

  validation {
    condition     = can(cidrnetmask(var.listener_network_cidr))
    error_message = "The listener_network_cidr must be a valid IPv4 CIDR, e.g. 10.20.2.0/24."
  }
}

variable "target_network_cidr" {
  description = "IPv4 prefix of the network that hosts the backend VMs. Must not overlap the listener network and, in a Network Area, must lie inside its network ranges."
  type        = string
  default     = "10.20.3.0/24"

  validation {
    condition     = can(cidrnetmask(var.target_network_cidr))
    error_message = "The target_network_cidr must be a valid IPv4 CIDR, e.g. 10.20.3.0/24."
  }
}

variable "availability_zones" {
  description = "Availability zones for the backend VMs. One VM is created per zone."
  type        = list(string)
  default     = ["eu01-1", "eu01-2"]

  validation {
    condition     = length(var.availability_zones) >= 1 && length(var.availability_zones) <= 3
    error_message = "Provide between one and three availability zones."
  }
}

variable "machine_type" {
  description = "Machine type of the backend VMs and the jump host."
  type        = string
  default     = "c2i.1"
}

variable "image_name" {
  description = "Name of the boot image for all VMs, resolved via the stackit_image_v2 data source. The image must ship python3 and openssl."
  type        = string
  default     = "Debian 12"
}

variable "boot_volume_size_gb" {
  description = "Boot volume size of each VM in GB."
  type        = number
  default     = 20
}

variable "alb_plan_id" {
  description = "Service plan of the Application Load Balancer. List the plans of your region with `stackit beta alb plans`; p10 is the only plan available at the time of writing."
  type        = string
  default     = "p10"
}

variable "admin_cidr" {
  description = "Source CIDR that may reach the jump host over SSH, e.g. your egress address as 203.0.113.10/32. Avoid 0.0.0.0/0."
  type        = string

  validation {
    condition     = can(cidrnetmask(var.admin_cidr))
    error_message = "The admin_cidr must be a valid IPv4 CIDR, e.g. 203.0.113.10/32."
  }
}

variable "ssh_public_key" {
  description = "SSH public key that is registered for the jump host, e.g. the content of ~/.ssh/id_ed25519.pub."
  type        = string

  validation {
    condition     = can(regex("^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp[0-9]+) ", var.ssh_public_key))
    error_message = "The ssh_public_key must be an OpenSSH public key (ssh-ed25519, ssh-rsa or ecdsa-sha2-*)."
  }
}
