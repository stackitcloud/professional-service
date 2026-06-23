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

resource "stackit_resourcemanager_project" "sfs-no-folder" {
  parent_container_id = var.STACKIT_ORG_ID
  name                = "sfs-example"
  labels = {
    "networkArea" = stackit_network_area.sfs.network_area_id
  }
  owner_email = "markus.brunsch@stackit.cloud"
}

resource "stackit_resourcemanager_project" "sfs-folder" {
  parent_container_id = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" #Folder ID Demos
  name                = "sfs-example-folder"
  labels = {
    "networkArea" = stackit_network_area.sfs.network_area_id
  }
  owner_email = "markus.brunsch@stackit.cloud"
}
