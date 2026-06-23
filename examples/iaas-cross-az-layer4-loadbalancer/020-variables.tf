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

variable "stackit_project_id" {
  type    = string
  default = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}

variable "stackit_region" {
  type    = string
  default = "eu01"
}

variable "stackit_service_account_key_path" {
  type    = string
  default = "../../keys/stackit-sa.json"
}

resource "stackit_key_pair" "admin_keypair" {
  name       = "admin-keypair-12345"
  public_key = chomp(file("~/.ssh/id_rsa.pub"))
}

variable "jumphost_flavor" {
  default = "c2i.1"
}
