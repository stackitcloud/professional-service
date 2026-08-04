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

variable "stackit_service_account_key_path" {
  description = "Path to the STACKIT service account key file (JSON). Keep this file out of version control."
  type        = string
  default     = "./keys/service-account.json"
}

variable "stackit_organization_id" {
  description = "STACKIT Organization ID (UUID)."
  type        = string
}

variable "stackit_network_area_id" {
  description = "Shared Network Area ID from the hub project. Run `terraform output network_area_id` in 001-hub-project."
  type        = string
}

variable "stackit_folder_id" {
  description = "STACKIT Folder ID (UUID) that will contain this spoke project."
  type        = string
}

variable "stackit_region" {
  description = "STACKIT region (e.g. eu01)."
  type        = string
  default     = "eu01"
}

variable "default_zone" {
  description = "Availability zone within the region (e.g. eu01-1)."
  type        = string
  default     = "eu01-1"
}

variable "project_name" {
  description = "Display name of this spoke project in STACKIT."
  type        = string
  default     = "spoke-project-02"
}

variable "org_admin" {
  description = "Email address of the STACKIT user who will be set as project owner."
  type        = string
}

variable "spoke_subnet" {
  description = "IPv4 prefix for this spoke's network. Must be within the network area range (10.28.0.0/16)."
  type        = string
  default     = "10.28.10.0/24"
}

variable "hub_firewall_lan_ip" {
  description = "LAN IP of the active OPNsense node. Used as the default route next-hop for all spoke traffic. Run `terraform output firewall_lan_ip` in 001-hub-project."
  type        = string
  default     = "10.28.1.100"
}
