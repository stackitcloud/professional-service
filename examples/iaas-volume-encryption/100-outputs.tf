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

output "volume_id" {
  value = stackit_volume.encrypted.volume_id
}

output "volume_encrypted" {
  value = stackit_volume.encrypted.encrypted
}

output "server_id" {
  value = stackit_server.this.server_id
}

output "kms_keyring_id" {
  value = stackit_kms_keyring.volume.keyring_id
}

output "kms_key_id" {
  value = stackit_kms_key.volume.key_id
}

output "kms_key_version" {
  value = var.kek_key_version
}

output "kms_service_account_email" {
  description = "Service account IaaS uses to access the key."
  value       = stackit_service_account.volume_kms.email
}

output "admin_password" {
  description = "Password of the local Administrator: terraform output -raw admin_password"
  value       = random_password.admin.result
  sensitive   = true
}
