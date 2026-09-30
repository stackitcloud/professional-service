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
  type    = string
  default = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}

variable "stackit_region" {
  type        = string
  default     = "eu01"
  description = "STACKIT region."
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the STACKIT service account key JSON file."
}

variable "cluster_name" {
  type        = string
  default     = "audit-demo"
  description = "Name of the SKE cluster."
}

variable "network_ipv4_prefix" {
  type        = string
  default     = "10.40.20.0/24"
  description = "IPv4 prefix of the SKE node network."
}

variable "network_nameservers" {
  type        = list(string)
  default     = ["9.9.9.9", "1.1.1.1"]
  description = "Nameservers for the SKE node network."
}

variable "observability_plan_name" {
  type        = string
  default     = "Observability-Large-EU01"
  description = "Plan of the Observability instance"
}

variable "observability_logs_retention_days" {
  type        = number
  default     = 7
  description = "Log retention of the Observability instance."
}

variable "telemetry_acl" {
  type        = list(string)
  default     = ["0.0.0.0/0"]
  description = "Access control list of the Observability/Logs."
}

variable "telemetry_link_token_version" {
  type        = number
  default     = 1
  description = "https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/telemetrylink#access_token_wo_version-4"
}

locals {
  common_labels = {
    "managed-by" = "terraform"
    "example"    = "ske-kubeapi-audit-log"
  }
}
