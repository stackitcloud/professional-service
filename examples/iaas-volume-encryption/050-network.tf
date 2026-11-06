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

resource "stackit_network" "this" {
  project_id       = var.stackit_project_id
  name             = "${var.name_prefix}-network"
  ipv4_prefix      = var.network_cidr
  ipv4_nameservers = var.network_nameservers
  labels           = local.labels
}

resource "stackit_security_group" "this" {
  project_id  = var.stackit_project_id
  name        = "${var.name_prefix}-sg"
  description = "Server with the encrypted boot volume"
  stateful    = true
  labels      = local.labels
}

resource "stackit_network_interface" "this" {
  project_id         = var.stackit_project_id
  network_id         = stackit_network.this.network_id
  name               = "${var.name_prefix}-nic"
  security_group_ids = [stackit_security_group.this.security_group_id]
  labels             = local.labels
}
