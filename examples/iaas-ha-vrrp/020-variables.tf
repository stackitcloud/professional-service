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
  type        = string
  description = "The STACKIT project ID to deploy resources into."
}

variable "stackit_region" {
  type        = string
  default     = "eu01"
  description = "The STACKIT region to deploy resources into."
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the STACKIT service account key JSON file used for provider authentication."
}

resource "stackit_key_pair" "admin_keypair" {
  name       = "vrrp-test-keypair"
  public_key = chomp(file("~/.ssh/id_rsa.pub"))
}


locals {
  user_data_active = templatefile("cloud-init.yaml", {
    type     = "MASTER"
    priority = "255"
    vip      = stackit_network_interface.vip01.ipv4
  })

  user_data_passive = templatefile("cloud-init.yaml", {
    type     = "BACKUP"
    priority = "254"
    vip      = stackit_network_interface.vip01.ipv4
  })
}
