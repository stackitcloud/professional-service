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

output "creds_project_id" {
  description = "Project ID of the credentials sub-project"
  value       = stackit_resourcemanager_project.creds.project_id
}

output "secretsmanager_instance_id" {
  description = "Instance ID of the Secrets Manager instance"
  value       = stackit_secretsmanager_instance.this.instance_id
}

output "admin_credentials_group_urn" {
  description = "URN of the admin credentials group"
  value       = stackit_objectstorage_credentials_group.admin.urn
}

output "write_credentials_group_urn" {
  description = "URN of the write-only credentials group"
  value       = stackit_objectstorage_credentials_group.write.urn
}

output "read_credentials_group_urn" {
  description = "URN of the read-only credentials group"
  value       = stackit_objectstorage_credentials_group.read.urn
}
