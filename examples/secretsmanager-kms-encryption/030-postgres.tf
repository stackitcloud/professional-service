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

# A small PostgreSQL Flex instance. The credentials of its database user
# are stored in the Secrets Manager instance; the round-trip script reads
# them back and connects with them.
resource "stackit_postgresflex_instance" "this" {
  project_id      = var.stackit_project_id
  name            = "${var.name_prefix}-postgres"
  version         = var.postgres_version
  flavor_id       = var.postgres_flavor_id
  backup_schedule = var.postgres_backup_schedule
  retention_days  = var.postgres_retention_days

  storage = {
    class = var.postgres_storage_class
    size  = var.postgres_storage_size
  }

  network = {
    acl = var.acls
  }
}

resource "stackit_postgresflex_user" "this" {
  project_id  = var.stackit_project_id
  instance_id = stackit_postgresflex_instance.this.instance_id
  username    = "app"
  roles       = ["login"]
}

resource "stackit_postgresflex_database" "this" {
  project_id  = var.stackit_project_id
  instance_id = stackit_postgresflex_instance.this.instance_id
  name        = "app"
  owner       = stackit_postgresflex_user.this.username
}
