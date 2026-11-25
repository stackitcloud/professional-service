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

resource "stackit_objectstorage_bucket" "this" {
  project_id = var.stackit_project_id
  name       = var.bucket_name
}

# Admin credentials group in the same project (retains full access to manage the bucket and policy)
resource "stackit_objectstorage_credentials_group" "admin" {
  project_id = var.stackit_project_id
  name       = "admin-credentials"

  depends_on = [stackit_objectstorage_bucket.this]
}

resource "stackit_objectstorage_credential" "admin" {
  project_id           = var.stackit_project_id
  credentials_group_id = stackit_objectstorage_credentials_group.admin.credentials_group_id
}

# Write-only credentials group in the same project
resource "stackit_objectstorage_credentials_group" "write" {
  project_id = var.stackit_project_id
  name       = "write-only-credentials"

  depends_on = [stackit_objectstorage_bucket.this]
}

resource "stackit_objectstorage_credential" "write" {
  project_id           = var.stackit_project_id
  credentials_group_id = stackit_objectstorage_credentials_group.write.credentials_group_id
}

# Read-only credentials group in the same project
resource "stackit_objectstorage_credentials_group" "read" {
  project_id = var.stackit_project_id
  name       = "read-only-credentials"

  depends_on = [stackit_objectstorage_bucket.this]
}

resource "stackit_objectstorage_credential" "read" {
  project_id           = var.stackit_project_id
  credentials_group_id = stackit_objectstorage_credentials_group.read.credentials_group_id
}

# Unprivileged credentials group in the same project (blocked by the default-deny policy)
resource "stackit_objectstorage_credentials_group" "denied" {
  project_id = var.stackit_project_id
  name       = "unprivileged-credentials"

  depends_on = [stackit_objectstorage_bucket.this]
}

resource "stackit_objectstorage_credential" "denied" {
  project_id           = var.stackit_project_id
  credentials_group_id = stackit_objectstorage_credentials_group.denied.credentials_group_id
}
