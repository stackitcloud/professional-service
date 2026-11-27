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

output "instance_id" {
  description = "ID of the Secrets Manager instance"
  value       = stackit_secretsmanager_instance.this.instance_id
}

output "keyring_id" {
  description = "ID of the KMS keyring"
  value       = stackit_kms_keyring.this.keyring_id
}

output "key_id" {
  description = "ID of the key encryption key"
  value       = stackit_kms_key.this.key_id
}

output "kms_key_version" {
  description = "Key version the instance is pinned to. Record it: the key material must stay usable for the whole lifetime of the instance."
  value       = var.kms_key_version
}

output "kms_service_account" {
  description = "Service account the Secrets Manager instance uses to unwrap the key"
  value       = stackit_service_account.secrets_manager_kms.email
}

output "postgres_instance_id" {
  description = "ID of the PostgreSQL Flex instance"
  value       = stackit_postgresflex_instance.this.instance_id
}

output "postgres_host" {
  description = "Endpoint of the PostgreSQL Flex instance"
  value       = try(stackit_postgresflex_instance.this.connection_info.write.host, null)
}

output "postgres_database" {
  description = "Name of the created database"
  value       = stackit_postgresflex_database.this.name
}

output "postgres_user" {
  description = "Name of the created database user"
  value       = stackit_postgresflex_user.this.username
}

output "postgres_password" {
  description = "Password of the created database user"
  value       = stackit_postgresflex_user.this.password
  sensitive   = true
}

output "sm_vault_address" {
  description = "Base URL of the Secrets Manager Vault API"
  value       = local.sm_vault_address
}

output "sm_secret_name" {
  description = "Name of the secret that holds the database credentials"
  value       = vault_kv_secret_v2.postgres_credentials.name
}

output "sm_username" {
  description = "Secrets Manager user the round-trip script logs in with"
  value       = stackit_secretsmanager_user.this.username
}

output "sm_password" {
  description = "Secrets Manager user password. The round-trip script reads it with `terraform output -raw`."
  value       = stackit_secretsmanager_user.this.password
  sensitive   = true
}
