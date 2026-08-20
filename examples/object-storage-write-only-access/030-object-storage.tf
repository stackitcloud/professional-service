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

resource "stackit_objectstorage_compliance_lock" "this" {
  project_id = var.stackit_project_id
}

resource "stackit_objectstorage_bucket" "this" {
  project_id = var.stackit_project_id
  name       = var.bucket_name

  object_lock = true

  depends_on = [stackit_objectstorage_compliance_lock.this]
}

data "stackit_resourcemanager_project" "base_project" {
  project_id = var.stackit_project_id
}

resource "stackit_resourcemanager_project" "creds" {
  parent_container_id = data.stackit_resourcemanager_project.base_project.parent_container_id
  name                = var.credentials_project_name
  owner_email         = var.owner_email
}

resource "stackit_objectstorage_credentials_group" "admin" {
  project_id = var.stackit_project_id
  name       = "admin-credentials"

  depends_on = [stackit_objectstorage_bucket.this]
}

resource "stackit_objectstorage_credential" "admin" {
  project_id           = var.stackit_project_id
  credentials_group_id = stackit_objectstorage_credentials_group.admin.credentials_group_id
}

resource "stackit_objectstorage_credentials_group" "write" {
  project_id = stackit_resourcemanager_project.creds.project_id
  name       = "write-only-credentials"

  depends_on = [stackit_objectstorage_bucket.this]
}

resource "stackit_objectstorage_credential" "write" {
  project_id           = stackit_resourcemanager_project.creds.project_id
  credentials_group_id = stackit_objectstorage_credentials_group.write.credentials_group_id
}

resource "stackit_objectstorage_credentials_group" "read" {
  project_id = stackit_resourcemanager_project.creds.project_id
  name       = "read-only-credentials"

  depends_on = [stackit_objectstorage_bucket.this]
}

resource "stackit_objectstorage_credential" "read" {
  project_id           = stackit_resourcemanager_project.creds.project_id
  credentials_group_id = stackit_objectstorage_credentials_group.read.credentials_group_id
}
