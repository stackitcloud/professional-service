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
  description = "The STACKIT project ID where all resources are deployed."
  type        = string
}

variable "stackit_region" {
  description = "The STACKIT region."
  type        = string
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  description = "Path to the STACKIT service account key JSON file."
  type        = string
  default     = "./keys/stackit-sa.json"
}

variable "cluster_name" {
  description = "Name of the SKE cluster."
  type        = string
  default     = "velerodemo"
}

variable "velero_backup_schedule" {
  description = "Cron expression for the default full-cluster backup schedule."
  type        = string
  default     = "0 2 * * *"
}

variable "velero_backup_ttl" {
  description = "Retention duration for backups in Velero's duration format (e.g. 720h0m0s = 30 days)."
  type        = string
  default     = "720h0m0s"
}
