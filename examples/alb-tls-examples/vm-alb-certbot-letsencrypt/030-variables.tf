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
  description = "STACKIT region (e.g. eu01)"
  type        = string
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  description = "Relative path to the STACKIT service account key JSON file"
  type        = string
  default     = "keys/sa-key.json"
}

# ─── Resource Hierarchy ───────────────────────────────────────────────────────

variable "organization_id" {
  description = "STACKIT organisation container ID — parent into which the folder is created (find in the STACKIT Portal under Organisation settings)"
  type        = string
}

variable "owner_email" {
  description = "Email of the resource owner; used for folder and project creation (must be an existing STACKIT user in the organisation)"
  type        = string
}

variable "folder_name" {
  description = "Name of the folder to create under the organisation"
  type        = string
  default     = "alb-certbot-workshop"
}

variable "project_name" {
  description = "Name of the project to create inside the folder"
  type        = string
  default     = "alb-certbot-dev"
}

# ─── Network ──────────────────────────────────────────────────────────────────

variable "network_name" {
  description = "Name of the private network"
  type        = string
  default     = "alb-certbot-net"
}

variable "network_cidr" {
  description = "IPv4 CIDR block for the network (e.g. 10.10.0.0/24)"
  type        = string
  default     = "10.10.0.0/24"

  validation {
    condition     = can(cidrnetmask(var.network_cidr))
    error_message = "network_cidr must be a valid CIDR notation, e.g. 10.10.0.0/24."
  }
}

variable "admin_cidr" {
  description = "Source CIDR allowed for SSH (port 22) — use your egress IP, e.g. 203.0.113.10/32. Avoid 0.0.0.0/0."
  type        = string

  validation {
    condition     = can(cidrnetmask(var.admin_cidr))
    error_message = "admin_cidr must be a valid CIDR notation, e.g. 203.0.113.10/32."
  }
}

# ─── Compute / VM ─────────────────────────────────────────────────────────────

variable "vm_name" {
  description = "Name of the Docker host VM"
  type        = string
  default     = "alb-certbot-docker-host"
}

variable "machine_type" {
  description = "STACKIT machine type — list available: stackit server machine-type list --project-id <id>"
  type        = string
  default     = "g1.1"
}

variable "availability_zone" {
  description = "Availability zone for the VM (e.g. eu01-1, eu01-2, eu01-3)"
  type        = string
  default     = "eu01-1"
}

variable "image_id" {
  description = "UUID of the boot image (Debian 12 recommended) — list available: stackit image list --all --project-id <id>"
  type        = string
}

variable "boot_volume_size_gb" {
  description = "Root disk size in GB"
  type        = number
  default     = 32

  validation {
    condition     = var.boot_volume_size_gb >= 10
    error_message = "boot_volume_size_gb must be at least 10 GB."
  }
}

variable "keypair_name" {
  description = "Name of the SSH key pair to register in STACKIT"
  type        = string
  default     = "alb-certbot-workshop-key"
}

variable "ssh_public_key" {
  description = "SSH public key string (ssh-ed25519 AAAA... or ssh-rsa AAAA...) — never commit the private key"
  type        = string
  sensitive   = true
}

variable "start_nginx_test_container" {
  description = "Start an nginx:alpine test container on port 80 via cloud-init (useful for ALB backend health-check validation)"
  type        = bool
  default     = true
}

# ─── DNS ──────────────────────────────────────────────────────────────────────

variable "dns_zone_name" {
  description = "Human-readable label for the DNS zone resource"
  type        = string
  default     = "alb-certbot-workshop-zone"
}

variable "dns_name" {
  description = "DNS zone apex FQDN, e.g. workshop.example.com — must be a domain you control or can delegate"
  type        = string
}

variable "dns_contact_email" {
  description = "SOA contact email for the DNS zone"
  type        = string
}

# ─── ALB (Option A) ───────────────────────────────────────────────────────────

variable "alb_name" {
  description = "Name of the Application Load Balancer"
  type        = string
  default     = "alb-certbot-workshop"
}

variable "alb_plan_id" {
  description = "ALB service plan — p10 is the smallest available plan"
  type        = string
  default     = "p10"
}


variable "alb_target_pool_name" {
  description = "Name of the ALB target pool pointing to the Docker host VM"
  type        = string
  default     = "docker-host-pool"
}

variable "alb_acl_ranges" {
  description = "List of source CIDRs allowed to reach the ALB. Use [\"0.0.0.0/0\"] to allow all traffic."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}
