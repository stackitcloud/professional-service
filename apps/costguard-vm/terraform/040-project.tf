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


# A dedicated project directly under the organization by default: whoever is
# owner, editor or Service Account User on it (or on a parent folder) can act
# as costguard-vm's service account, so nobody but the deployer gets a role
# here. owner_email becomes owner: the deployer service account.
resource "stackit_resourcemanager_project" "costguard" {
  count = local.create_project ? 1 : 0

  parent_container_id = coalesce(var.parent_container_id, var.organization_id)
  name                = "costguard"
  owner_email         = var.project_owner_email
  labels              = local.labels
}
