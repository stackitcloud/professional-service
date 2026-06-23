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

# Get vNET Networks
resource "stackit_network_area" "sfs" {
  organization_id = var.STACKIT_ORG_ID
  name            = "sfs-network-area"
  labels = {
    "preview/routingtables" = "true"
  }
}

resource "stackit_network_area_region" "sfs" {
  organization_id = var.STACKIT_ORG_ID
  network_area_id = stackit_network_area.sfs.network_area_id
  ipv4 = {
    transfer_network = "10.1.2.0/24"
    network_ranges = [
      {
        prefix = "10.0.0.0/16"
      }
    ]
  }
}
