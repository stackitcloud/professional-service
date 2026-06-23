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

resource "stackit_resourcemanager_folder" "showcase" {
  name                = var.folder_name
  owner_email         = var.owner_email
  parent_container_id = var.organization_id

  labels = {
    environment = "workshop"
    managed-by  = "terraform"
    use-case    = "alb-certbot"
  }
}

resource "stackit_resourcemanager_project" "showcase" {
  name                = var.project_name
  owner_email         = var.owner_email
  parent_container_id = stackit_resourcemanager_folder.showcase.id

  labels = {
    environment = "workshop"
    managed-by  = "terraform"
    folder      = var.folder_name
    use-case    = "alb-certbot"
  }
}
