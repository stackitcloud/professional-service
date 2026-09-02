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
  type        = string
  description = "STACKIT project ID to deploy resources into."
}

variable "stackit_region" {
  type        = string
  description = "STACKIT region to deploy resources in."
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the STACKIT service account key file used for authentication."
}

variable "juicefs_filesystem_name" {
  type        = string
  description = "Name of the JuiceFS filesystem. Used when formatting the filesystem on first use."
  default     = "stackit-juicefs"
}
