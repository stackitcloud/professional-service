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

# ─── Provider ─────────────────────────────────────────────────────────────────

variable "stackit_region" {
  description = "STACKIT region, e.g. eu01"
  type        = string
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  description = "Path to the STACKIT service account key JSON file"
  type        = string
  default     = "keys/sa-key.json"
}

# ─── Project ──────────────────────────────────────────────────────────────────

variable "project_id" {
  description = "STACKIT project ID to which the image is uploaded — find it in the Portal or via: stackit project list"
  type        = string
}

# ─── Image ────────────────────────────────────────────────────────────────────

variable "image_name" {
  description = "Name of the custom image as it will appear in STACKIT"
  type        = string
}

variable "image_file_path" {
  description = "Local path to the image file to upload, e.g. ./images/custom-image.qcow2 — the file must exist before running terraform apply"
  type        = string
}

variable "disk_format" {
  description = "Disk format of the image. Supported values: qcow2, raw, iso"
  type        = string
  default     = "qcow2"

  validation {
    condition     = contains(["qcow2", "raw", "iso"], var.disk_format)
    error_message = "disk_format must be one of: qcow2, raw, iso."
  }
}

variable "min_disk_size" {
  description = "Minimum disk size required to boot instances from this image, in GB"
  type        = number
  default     = 20

  validation {
    condition     = var.min_disk_size >= 1
    error_message = "min_disk_size must be at least 1 GB."
  }
}

variable "min_ram" {
  description = "Minimum RAM required to boot instances from this image, in MB"
  type        = number
  default     = 2048

  validation {
    condition     = var.min_ram >= 1
    error_message = "min_ram must be at least 1 MB."
  }
}

# ─── Image Config ─────────────────────────────────────────────────────────────

variable "uefi" {
  description = "Enable UEFI boot for instances created from this image. Set to false for BIOS-only images."
  type        = bool
  default     = true
}

variable "secure_boot" {
  description = "Enable Secure Boot for instances created from this image. Requires UEFI (uefi = true)."
  type        = bool
  default     = false
}

# ─── Server ───────────────────────────────────────────────────────────────────

variable "server_name" {
  description = "Name of the server to create from the uploaded image"
  type        = string
  default     = "custom-image-server"
}

variable "machine_type" {
  description = "STACKIT machine type for the server — list available: stackit server machine-type list --project-id <id>"
  type        = string
  default     = "g1.1"
}

variable "availability_zone" {
  description = "Availability zone for the server, e.g. eu01-1, eu01-2, eu01-3"
  type        = string
  default     = "eu01-1"
}

variable "boot_volume_size_gb" {
  description = "Boot volume size in GB — must be >= min_disk_size of the image"
  type        = number
  default     = 20

  validation {
    condition     = var.boot_volume_size_gb >= 1
    error_message = "boot_volume_size_gb must be at least 1 GB."
  }
}

variable "network_cidr" {
  description = "IPv4 CIDR block for the server's private network"
  type        = string
  default     = "10.10.0.0/24"

  validation {
    condition     = can(cidrnetmask(var.network_cidr))
    error_message = "network_cidr must be a valid CIDR, e.g. 10.10.0.0/24."
  }
}

variable "keypair_name" {
  description = "Name of the SSH key pair to register in STACKIT"
  type        = string
  default     = "custom-image-key"
}

variable "ssh_public_key" {
  description = "SSH public key string (ssh-ed25519 AAAA... or ssh-rsa AAAA...) — only required when deploy_server = true"
  type        = string
  sensitive   = true
  default     = ""
}

# ─── Labels ───────────────────────────────────────────────────────────────────

variable "labels" {
  description = "Key-value labels to attach to the image resource"
  type        = map(string)
  default = {
    managed-by = "terraform"
    example    = "image-upload"
  }
}
