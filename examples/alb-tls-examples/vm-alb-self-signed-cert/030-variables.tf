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

# ─── Resource Hierarchy ───────────────────────────────────────────────────────

variable "organization_id" {
  description = "STACKIT organisation container ID — find it in the Portal under Organisation → Settings"
  type        = string
}

variable "owner_email" {
  description = "Email of the resource owner; must be an existing STACKIT user in the organisation"
  type        = string
}

variable "folder_name" {
  description = "Display name of the folder (must match the existing folder name when importing)"
  type        = string
  default     = "alb-showcase"
}

variable "project_name" {
  description = "Name of the new project to create inside the folder"
  type        = string
  default     = "vm-alb-self-signed-cert"
}

# ─── Naming ───────────────────────────────────────────────────────────────────

variable "name_prefix" {
  description = "Short prefix applied to all resource names (network, VMs, ALB, certificate)"
  type        = string
  default     = "vm-alb-tls"
}

# ─── Network ──────────────────────────────────────────────────────────────────

variable "network_cidr" {
  description = "IPv4 CIDR block for the private network, e.g. 10.10.0.0/24"
  type        = string
  default     = "10.10.0.0/24"

  validation {
    condition     = can(cidrnetmask(var.network_cidr))
    error_message = "network_cidr must be a valid CIDR, e.g. 10.10.0.0/24."
  }
}

variable "admin_cidr" {
  description = "Source CIDR for SSH access (port 22). Use your own egress IP, e.g. 203.0.113.10/32. Avoid 0.0.0.0/0."
  type        = string

  validation {
    condition     = can(cidrnetmask(var.admin_cidr))
    error_message = "admin_cidr must be a valid CIDR, e.g. 203.0.113.10/32."
  }
}

# ─── Compute / VM ─────────────────────────────────────────────────────────────

variable "machine_type" {
  description = "STACKIT machine type for backend VMs — list available: stackit server machine-type list --project-id <id>"
  type        = string
  default     = "g1.1"
}

variable "availability_zone" {
  description = "Availability zone for the VMs, e.g. eu01-1, eu01-2, eu01-3"
  type        = string
  default     = "eu01-1"
}

variable "image_id" {
  description = "UUID of the boot image (Debian 12 recommended) — list: stackit image list --all --project-id <id>"
  type        = string
}

variable "boot_volume_size_gb" {
  description = "Root disk size in GB for each backend VM"
  type        = number
  default     = 20

  validation {
    condition     = var.boot_volume_size_gb >= 10
    error_message = "boot_volume_size_gb must be at least 10 GB."
  }
}

variable "keypair_name" {
  description = "Name of the SSH key pair to register in STACKIT"
  type        = string
  default     = "vm-alb-tls-key"
}

variable "ssh_public_key" {
  description = "SSH public key string (ssh-ed25519 AAAA... or ssh-rsa AAAA...) — never commit the private key"
  type        = string
  sensitive   = true
}

# ─── TLS / Certificate ────────────────────────────────────────────────────────

variable "tls_common_name" {
  description = "Common Name (CN) for the self-signed certificate, e.g. alb.example.com or the ALB IP address"
  type        = string
  default     = "alb.example.internal"
}

variable "tls_organization" {
  description = "Organisation name embedded in the certificate subject"
  type        = string
  default     = "STACKIT ALB Showcase"
}

variable "tls_validity_hours" {
  description = "Certificate validity in hours (8760 = 1 year)"
  type        = number
  default     = 8760
}

# ─── ALB ──────────────────────────────────────────────────────────────────────

variable "alb_plan_id" {
  description = "ALB service plan — p10 is the smallest available plan"
  type        = string
  default     = "p10"
}

variable "alb_acl_ranges" {
  description = "Source CIDRs allowed to reach the ALB. Use [\"0.0.0.0/0\"] for public access."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

# ─── DNS ──────────────────────────────────────────────────────────────────────

variable "dns_zone_name" {
  description = "Human-readable label for the DNS zone resource"
  type        = string
  default     = "vm-alb-tls-zone"
}

variable "dns_name" {
  description = "DNS zone apex FQDN, e.g. vm-alb-tls.stackit.gg"
  type        = string
}

variable "dns_contact_email" {
  description = "SOA contact email for the DNS zone"
  type        = string
}
