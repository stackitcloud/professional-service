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

resource "stackit_resourcemanager_folder" "folder_lvl_1" {
  name                = "folder_lvl_1"
  owner_email         = var.owner_email
  parent_container_id = var.stackit_org_id
}

resource "stackit_resourcemanager_folder" "folder_lvl_2" {
  name                = "folder_lvl_2"
  owner_email         = var.owner_email
  parent_container_id = stackit_resourcemanager_folder.folder_lvl_1.container_id
}

resource "stackit_resourcemanager_project" "proj_folder_1" {
  parent_container_id = stackit_resourcemanager_folder.folder_lvl_1.container_id
  name                = "proj-folder1"
  owner_email         = var.owner_email
}

resource "stackit_resourcemanager_project" "proj_folder_2" {
  parent_container_id = stackit_resourcemanager_folder.folder_lvl_2.container_id
  name                = "proj-folder2"
  owner_email         = var.owner_email
}


resource "stackit_authorization_project_role_assignment" "editor_folder_1" {
  resource_id = stackit_resourcemanager_project.proj_folder_2.project_id
  role        = "editor"
  subject     = "foo.bar@digits.schwarz"
}
