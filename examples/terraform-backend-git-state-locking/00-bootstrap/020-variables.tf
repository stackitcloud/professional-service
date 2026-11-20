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
  type = string
}

variable "stackit_parent_id" {
  description = "Organization or Folder"
  type        = string
}

variable "stackit_project_name" {
  description = "Display name of the STACKIT project that hosts the Git instance."
  type        = string
}

variable "stackit_org_admin" {
  description = "Email address"
  type        = string
}

variable "stackit_region" {
  type = string
}

variable "git_instance_name" {
  type = string
}

variable "git_flavor" {
  type = string
}

variable "git_acl" {
  description = "CIDR ranges"
  type        = list(string)

  validation {
    condition     = length(var.git_acl) > 0
    error_message = "Provide at least one CIDR range."
  }
}
