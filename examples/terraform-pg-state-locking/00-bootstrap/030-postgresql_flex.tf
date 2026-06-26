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

# Provision PostgreSQL Flex Database Instance
resource "stackit_postgresflex_instance" "this" {
  project_id = stackit_resourcemanager_project.this.project_id
  name       = "tf-state-instance"
  version    = "17"

  flavor = {
    cpu = 2
    ram = 4
  }
  storage = {
    class = "premium-perf2-stackit"
    size  = 10
  }

  replicas        = 1
  backup_schedule = "00 00 * * *"

  acl = [
    # WARNING: Open ACL is for development only. Restrict to your specific egress IP ranges in production.
    "0.0.0.0/0"
  ]

}

# Provision PostgreSQL Flex Database Owner
resource "stackit_postgresflex_user" "db_owner" {
  project_id  = stackit_resourcemanager_project.this.project_id
  instance_id = stackit_postgresflex_instance.this.instance_id
  username    = "tf_state_user"
  roles       = ["login", "createdb"]
}

# Provision PostgreSQL Flex Database
resource "stackit_postgresflex_database" "this" {
  project_id  = stackit_resourcemanager_project.this.project_id
  instance_id = stackit_postgresflex_instance.this.instance_id
  owner       = stackit_postgresflex_user.db_owner.username
  name        = "tf-states"
}
