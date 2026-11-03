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

# 03-migration reads these through terraform_remote_state.

output "project_id" {
  description = "Target project"
  value       = stackit_resourcemanager_project.target.project_id
}

output "instance_id" {
  description = "ID of the PostgreSQL Flex instance"
  value       = stackit_postgresflex_instance.target.instance_id
}

output "host" {
  description = "DNS name of the instance"
  value       = stackit_postgresflex_instance.target.connection_info.write.host
}

output "port" {
  description = "Port of the instance"
  value       = stackit_postgresflex_instance.target.connection_info.write.port
}

output "owner_user" {
  description = "Owner of all objects, pgcopydb writes with it"
  value       = stackit_postgresflex_user.owner.username
}

output "owner_password" {
  description = "Password of the owner"
  value       = stackit_postgresflex_user.owner.password
  sensitive   = true
}

output "app_users" {
  description = "Users the application connects with"
  value       = sort(keys(stackit_postgresflex_user.app))
}

output "app_user_passwords" {
  description = "Passwords of the application users, by name"
  value       = { for name, u in stackit_postgresflex_user.app : name => u.password }
  sensitive   = true
}

output "databases" {
  description = "Databases created on the target"
  value       = sort(keys(stackit_postgresflex_database.db))
}

output "acl" {
  description = "Addresses allowed to connect"
  value       = local.acl
}

output "backup_schedule" {
  description = "Backup schedule (UTC). Keep the migration window clear of it."
  value       = var.backup_schedule
}

output "storage_size_gb" {
  description = "Storage size of the instance"
  value       = var.storage_size_gb
}

output "postgres_version" {
  description = "Major version of the target"
  value       = var.postgres_version
}
