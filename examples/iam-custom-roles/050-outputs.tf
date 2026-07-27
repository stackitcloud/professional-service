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

output "service_account_email" {
  description = "Email of the created service account."
  value       = stackit_service_account.this.email
}

output "custom_role_name" {
  description = "Name of the custom IAM role."
  value       = stackit_authorization_project_custom_role.readonly.name
}

output "service_account_key" {
  description = "Service account key credentials. Use this to authenticate as the service account."
  value       = stackit_service_account_key.this.key_origin
  sensitive   = true
}
