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

resource "stackit_secretsmanager_instance" "this" {
  project_id = stackit_resourcemanager_project.creds.project_id
  name       = "s3-credentials-vault"
}

resource "stackit_secretsmanager_user" "this" {
  project_id    = stackit_resourcemanager_project.creds.project_id
  instance_id   = stackit_secretsmanager_instance.this.instance_id
  description   = "Terraform Vault User for S3 Credentials"
  write_enabled = true
}
