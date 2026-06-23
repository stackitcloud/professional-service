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

resource "stackit_postgresflex_instance" "this" {
  project_id      = var.stackit_project_id
  name            = "example-instance"
  backup_schedule = "00 00 * * *"
  flavor = {
    cpu = 2
    ram = 4
  }
  replicas = 3
  storage = {
    class = "premium-perf2-stackit"
    size  = 15
  }
  version = 15
  acl     = ["0.0.0.0/0"]
}

resource "stackit_postgresflex_user" "this" {
  project_id  = var.stackit_project_id
  instance_id = stackit_postgresflex_instance.this.instance_id
  username    = "test"
  roles       = ["createdb", "login"]
}

resource "stackit_postgresflex_database" "this" {
  project_id  = var.stackit_project_id
  instance_id = stackit_postgresflex_instance.this.instance_id
  name        = "test"
  owner       = stackit_postgresflex_user.this.username
}
