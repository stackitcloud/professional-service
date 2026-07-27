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

# Object Storage bucket names must be globally unique across all STACKIT tenants.
# A random suffix guarantees uniqueness without requiring manual coordination.
resource "random_pet" "bucket_suffix" {
  length = 2
}

resource "stackit_objectstorage_bucket" "velero" {
  project_id = var.stackit_project_id
  name       = "velero-backups-${random_pet.bucket_suffix.id}"
}

resource "stackit_objectstorage_credentials_group" "velero" {
  project_id = var.stackit_project_id
  name       = "velero-credentials"

  depends_on = [stackit_objectstorage_bucket.velero]
}

resource "stackit_objectstorage_credential" "velero" {
  project_id           = var.stackit_project_id
  credentials_group_id = stackit_objectstorage_credentials_group.velero.credentials_group_id
}
