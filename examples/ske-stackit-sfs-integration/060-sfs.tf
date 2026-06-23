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

resource "stackit_sfs_resource_pool" "no-folder-resourcepool" {
  project_id        = stackit_resourcemanager_project.sfs-no-folder.project_id
  name              = "sfs-resourcepool"
  availability_zone = "eu01-m"
  performance_class = "Standard"
  size_gigabytes    = 512
  ip_acl = [
    "0.0.0.0/0",
  ]
  snapshots_are_visible = true
}

resource "stackit_sfs_export_policy" "no-folder-policy" {
  project_id = stackit_resourcemanager_project.sfs-no-folder.project_id
  name       = "example"
  rules = [
    {
      ip_acl = ["0.0.0.0/0"]
      order  = 1
    }
  ]
}

resource "stackit_sfs_share" "no-folder-share" {
  project_id                 = stackit_resourcemanager_project.sfs-no-folder.project_id
  resource_pool_id           = stackit_sfs_resource_pool.no-folder-resourcepool.resource_pool_id
  name                       = "nfs-share"
  export_policy              = "example"
  space_hard_limit_gigabytes = 128
}

##############

resource "stackit_sfs_resource_pool" "folder-resourcepool" {
  project_id        = stackit_resourcemanager_project.sfs-folder.project_id
  name              = "sfs-resourcepool"
  availability_zone = "eu01-m"
  performance_class = "Standard"
  size_gigabytes    = 512
  ip_acl = [
    "0.0.0.0/0",
  ]
  snapshots_are_visible = true
}

resource "stackit_sfs_export_policy" "folder-policy" {
  project_id = stackit_resourcemanager_project.sfs-folder.project_id
  name       = "example"
  rules = [
    {
      ip_acl = ["0.0.0.0/0"]
      order  = 1
    }
  ]
}

resource "stackit_sfs_share" "folder-share" {
  project_id                 = stackit_resourcemanager_project.sfs-folder.project_id
  resource_pool_id           = stackit_sfs_resource_pool.folder-resourcepool.resource_pool_id
  name                       = "nfs-share"
  export_policy              = "example"
  space_hard_limit_gigabytes = 128
}

output "mount" {
  value = stackit_sfs_share.no-folder-share.mount_path
}
