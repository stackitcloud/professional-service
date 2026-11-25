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

output "bucket_name" {
  description = "Name of the Object Storage bucket"
  value       = stackit_objectstorage_bucket.this.name
}

output "s3_endpoint" {
  description = "S3 endpoint URL for STACKIT Object Storage"
  value       = "https://object.storage.${var.stackit_region}.onstackit.cloud"
}

output "admin_credentials_group_urn" {
  description = "URN of the admin credentials group"
  value       = stackit_objectstorage_credentials_group.admin.urn
}

output "admin_access_key" {
  description = "Access key for the admin credentials"
  value       = stackit_objectstorage_credential.admin.access_key
}

output "admin_secret_access_key" {
  description = "Secret access key for the admin credentials"
  value       = stackit_objectstorage_credential.admin.secret_access_key
  sensitive   = true
}

output "write_credentials_group_urn" {
  description = "URN of the write-only credentials group"
  value       = stackit_objectstorage_credentials_group.write.urn
}

output "write_access_key" {
  description = "Access key for the write-only credentials"
  value       = stackit_objectstorage_credential.write.access_key
}

output "write_secret_access_key" {
  description = "Secret access key for the write-only credentials"
  value       = stackit_objectstorage_credential.write.secret_access_key
  sensitive   = true
}

output "read_credentials_group_urn" {
  description = "URN of the read-only credentials group"
  value       = stackit_objectstorage_credentials_group.read.urn
}

output "read_access_key" {
  description = "Access key for the read-only credentials"
  value       = stackit_objectstorage_credential.read.access_key
}

output "read_secret_access_key" {
  description = "Secret access key for the read-only credentials"
  value       = stackit_objectstorage_credential.read.secret_access_key
  sensitive   = true
}

output "denied_credentials_group_urn" {
  description = "URN of the unprivileged credentials group (denied by default)"
  value       = stackit_objectstorage_credentials_group.denied.urn
}

output "denied_access_key" {
  description = "Access key for the unprivileged credentials"
  value       = stackit_objectstorage_credential.denied.access_key
}

output "denied_secret_access_key" {
  description = "Secret access key for the unprivileged credentials"
  value       = stackit_objectstorage_credential.denied.secret_access_key
  sensitive   = true
}
