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

variable "stackit_parent_container_id" {
  type        = string
  description = "Organization or folder the target project is created in"
}

variable "stackit_admin_email" {
  type        = string
  description = "Email address that becomes the owner of the created project"
}

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

variable "source_egress_cidrs" {
  type        = list(string)
  description = "Addresses the migration host uses towards the internet. null reads the public IP of the source VM from 01-source."
  default     = null
}

variable "databases" {
  type        = list(string)
  description = "Databases to create on the target. null takes the list of 01-source."
  default     = null
}

variable "extra_acl" {
  type        = list(string)
  description = "Additional CIDRs allowed to connect"
  default     = []
}

variable "project_name" {
  type        = string
  description = "Name of the target project"
  default     = "pg-migration-target"
}

variable "instance_name" {
  type    = string
  default = "pg-migration-target"
}

variable "postgres_version" {
  type    = string
  default = "18"
}

variable "flavor_id" {
  type        = string
  description = "Flavor of the PostgreSQL Flex instance, e.g. 2.4 or 2.4-replica"
  default     = "2.4"
}

variable "storage_class" {
  type        = string
  description = "Storage class of the pgflex instance"
  default     = "premium-perf2-stackit"
}

variable "storage_size_gb" {
  type        = number
  description = "in GB"
  default     = 20
}

# Do not start a migration while this runs. 03-migration shows it in preflight.
variable "backup_schedule" {
  type        = string
  description = "Cron expression (UTC) of the daily backup"
  default     = "0 2 * * *"
}

variable "backup_retention_days" {
  type    = number
  default = 32
}

variable "owner_user" {
  type        = string
  description = "User that owns the databases and every object in them. pgcopydb writes with it."
  default     = "app_owner"
}

# Same names as the roles on the source: 50-post migration script applies the grants of a source
# role to the user of the same name.
variable "app_users" {
  type        = list(string)
  description = "Users the application connects with. They own nothing and get the grants of the source role of the same name."
  default     = ["app_rw", "app_ro"]

  validation {
    condition     = !contains(var.app_users, var.owner_user)
    error_message = "owner_user must not be one of app_users."
  }
}
