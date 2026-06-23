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

variable "service_account_key_path" {
  type        = string
  description = "Path to the STACKIT Service Account JSON key file (e.g. 'keys/sa-key.json')."
}

variable "region" {
  type        = string
  description = "STACKIT region."
}

variable "organization_id" {
  type        = string
  description = "STACKIT Organization ID (parent of the folder)."
}

variable "folder_name" {
  type        = string
  description = "Name for the STACKIT folder."
}

variable "owner_email" {
  type        = string
  description = "Project owner email address."
}

variable "project_name" {
  type        = string
  description = "Name for the new STACKIT project."
}

variable "cluster_name" {
  type        = string
  description = "SKE cluster name. Maximum 11 characters."
}

variable "kubernetes_version_min" {
  type        = string
  description = "Minimum Kubernetes version. STACKIT auto-updates within this minor version."
}

variable "node_machine_type" {
  type        = string
  description = "Worker node machine type (e.g. 'g2i.2' = 2 vCPU / 8 GB RAM)."
}

variable "node_os_name" {
  type        = string
  description = "Node OS. SKE supports 'flatcar'."
}

variable "node_min" {
  type        = string
  description = "Minimum node count."
}

variable "node_max" {
  type        = string
  description = "Maximum node count."
}

variable "availability_zone" {
  type        = string
  description = "Availability zone for the node pool."
}

variable "dns_zone_name" {
  type        = string
  description = "STACKIT DNS zone display name."
}

variable "dns_zone_fqdn" {
  type        = string
  description = "DNS zone FQDN (e.g. 'showcase.example.com'). Must be a domain you control."
}

variable "dns_contact_email" {
  type        = string
  description = "Contact email for the DNS zone."
}

variable "app_hostname" {
  type        = string
  description = "Hostname label for the nginx app (e.g. 'nginx' → nginx.showcase.example.com)."
}
