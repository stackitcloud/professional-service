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

resource "stackit_objectstorage_bucket" "example" {
  project_id = var.project_id
  name       = "my-stackit-s3-bucket"
}

resource "stackit_objectstorage_credentials_group" "example" {
  project_id = var.project_id
  name       = "my-credentials-group"
}

resource "stackit_objectstorage_credential" "example" {
  project_id           = var.project_id
  credentials_group_id = stackit_objectstorage_credentials_group.example.credentials_group_id
}
