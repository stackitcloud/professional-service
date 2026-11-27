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


variable "stackit_region" {
  type    = string
  default = "eu01"
}

variable "stackit_service_account_key_path" {
  type    = string
  default = null
}

variable "source_state_path" {
  type        = string
  description = "State file of 01-source"
  default     = "../01-source/terraform.tfstate"
}

variable "target_state_path" {
  type        = string
  description = "State file of 02-postgres-flex"
  default     = "../02-postgres-flex/terraform.tfstate"
}

variable "ssh_private_key_path" {
  type = string
}

# pgcopydb config

variable "table_jobs" {
  type        = number
  description = "Tables copied in parallel"
  default     = 4
}

variable "index_jobs" {
  type        = number
  description = "Indexes built in parallel on the target"
  default     = 2
}

variable "split_tables_larger_than" {
  type        = string
  description = "Tables above this size are copied in parallel parts"
  default     = "200MB"
}
