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

locals {
  # The target-side counterpart of the enterprise firewall: only the migration
  # host may connect.
  acl = distinct(concat(local.source_egress_cidrs, var.extra_acl))
}

resource "stackit_postgresflex_instance" "target" {
  project_id = stackit_resourcemanager_project.target.project_id
  name       = var.instance_name
  version    = var.postgres_version

  flavor_id = var.flavor_id

  storage = {
    class = var.storage_class
    size  = var.storage_size_gb
  }

  network = {
    acl = local.acl
  }

  backup_schedule = var.backup_schedule
  retention_days  = var.backup_retention_days

  lifecycle {
    precondition {
      condition     = !contains(local.acl, "0.0.0.0/0")
      error_message = "The ACL must not contain 0.0.0.0/0."
    }
  }
}

# Owner of the databases and of every object in them. pgcopydb writes with this
# user (--no-owner), so everything on the target belongs to it. Its credentials
# are for the migration and for schema changes, not for the application.
resource "stackit_postgresflex_user" "owner" {
  project_id  = stackit_resourcemanager_project.target.project_id
  instance_id = stackit_postgresflex_instance.target.instance_id
  username    = var.owner_user
  roles       = ["login", "createdb"]
}

# The users the application connects with. They own nothing, so they cannot
# drop or alter tables. Migration Scripts gives them the grants they had on the source.
resource "stackit_postgresflex_user" "app" {
  for_each = toset(var.app_users)

  project_id  = stackit_resourcemanager_project.target.project_id
  instance_id = stackit_postgresflex_instance.target.instance_id
  username    = each.key
  roles       = ["login"]
}

# pgcopydb copies into an existing database. It does not create one.
resource "stackit_postgresflex_database" "db" {
  for_each = toset(local.databases)

  project_id  = stackit_resourcemanager_project.target.project_id
  instance_id = stackit_postgresflex_instance.target.instance_id
  name        = each.key
  owner       = stackit_postgresflex_user.owner.username
}
