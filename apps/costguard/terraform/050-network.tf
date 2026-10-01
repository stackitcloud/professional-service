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


resource "stackit_network" "costguard" {
  count = local.create_network ? 1 : 0

  project_id       = local.project_id
  name             = "costguard"
  ipv4_prefix      = var.network_ipv4_prefix
  ipv4_nameservers = var.dns_nameservers
  labels           = local.labels
}

resource "stackit_security_group" "costguard" {
  project_id  = local.project_id
  name        = "costguard"
  description = "costguard: egress only"
  stateful    = true
  labels      = local.labels
}

resource "stackit_network_interface" "costguard" {
  project_id         = local.project_id
  network_id         = local.network_id
  name               = "costguard"
  security_group_ids = [stackit_security_group.costguard.security_group_id]
  labels             = local.labels
}
