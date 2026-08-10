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

variable "stackit_admin_email" {
  description = "The email address of the project administrator."
  type        = string
  validation {
    condition     = can(regex("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", var.stackit_admin_email))
    error_message = "The stackit_admin_email must be a valid email address."
  }
}

variable "stackit_project_name" {
  description = "The name of the STACKIT project that hosts the SKE cluster and the ALBs."
  type        = string
  default     = "ske-alb-self-installed"
}

variable "stackit_region" {
  type    = string
  default = "eu01"
}

variable "stackit_service_account_key_path" {
  description = "Path to the service account key used by the STACKIT provider. The service account needs organization-level permissions to create the SNA and the project."
  type        = string
}

variable "sna_name" {
  description = "The name of the STACKIT Network Area (SNA)."
  type        = string
  default     = "ske-alb-self-installed-sna"
}

variable "sna_transfer_range" {
  description = "The STACKIT SNA transfer range in CIDR notation."
  type        = string
  default     = "172.16.10.0/24"
}

variable "sna_network_range" {
  description = "The STACKIT SNA network range in CIDR notation."
  type        = string
  default     = "10.40.0.0/16"
}

variable "sna_default_nameserver" {
  description = "A list of STACKIT SNA default nameservers."
  type        = list(string)
  default     = ["1.1.1.1"]
}

variable "network_ipv4_prefix" {
  description = "The IPv4 prefix of the SKE node network (must be a subnet of the SNA network range)."
  type        = string
  default     = "10.40.10.0/24"
}

variable "alb_controller_image" {
  description = "Container image of the STACKIT Application Load Balancer Controller."
  type        = string
  default     = "ghcr.io/stackitcloud/application-load-balancer-controller:v0.1.0"
}

variable "test_app_hostname" {
  description = "Hostname used for the demo Ingress and the self-signed TLS certificate. Does not need to be a registered DNS name; verification uses curl --resolve."
  type        = string
  default     = "alb-demo.example.com"
}
