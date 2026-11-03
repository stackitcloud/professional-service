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

#
# Custom User Settings
#

# No default on purpose: Terraform asks for this value if you forget it.
variable "stackit_project_id" {
  type        = string
  description = "STACKIT project ID where the Secrets Manager instance, the KMS keyring and key, and the service account are created. Find it with `stackit project list` or in the portal."
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the service account key file used by Terraform. Keep it out of version control."
  default     = "./keys/stackit-sa.json"
}

variable "stackit_region" {
  type        = string
  description = "STACKIT region. Secrets Manager is only available in eu01."
  default     = "eu01"
}

variable "name_prefix" {
  type        = string
  description = "Name prefix for the instance, keyring, key and service account"
  default     = "sm-kms-example"
}

# No default on purpose: a wrong ACL either locks you out or exposes the instance.
variable "acls" {
  type        = list(string)
  description = "Networks allowed to reach the Secrets Manager instance and the PostgreSQL Flex instance, e.g. [\"203.0.113.10/32\"]. Never 0.0.0.0/0 in production."
}

variable "kms_key_version" {
  type        = number
  description = "Key version the instance uses for encryption. The key is created with version 1."
  default     = 1
}

variable "postgres_flavor_id" {
  type        = string
  description = "Size of the PostgreSQL Flex instance. List them with: stackit postgresflex flavor list"
  default     = "2.4"
}

variable "postgres_storage_class" {
  type        = string
  description = "Storage class of the PostgreSQL Flex instance"
  default     = "premium-perf2-stackit"
}

variable "postgres_storage_size" {
  type        = number
  description = "Storage size of the PostgreSQL Flex instance in GB"
  default     = 10
}

variable "postgres_version" {
  type        = string
  description = "PostgreSQL major version"
  default     = "17"
}

variable "postgres_backup_schedule" {
  type        = string
  description = "Cron expression in UTC, when the daily backup runs"
  default     = "0 2 * * *"
}

variable "postgres_retention_days" {
  type        = number
  description = "How long backups are kept. Allowed range is 32 to 90 days."
  default     = 32
}
