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


resource "stackit_network_area" "sna" {
  organization_id = var.organization_id
  name            = "paloalto-ha"
}

resource "stackit_network_area_region" "sna1" {
  organization_id = var.organization_id
  network_area_id = stackit_network_area.sna.network_area_id
  region          = var.default_region
  ipv4 = {
    transfer_network = "192.168.254.0/24"
    network_ranges = [
      {
        prefix = "10.220.0.0/16"
      }
    ]
    default_nameservers   = ["1.1.1.1", "8.8.8.8"]
    default_prefix_length = 25
    max_prefix_length     = 29
    min_prefix_length     = 24
  }
}

// Projects are deleted asynchronously. On destroy, wait after the project is gone
// before deleting the network area region, otherwise it fails with "has still active projects".
// Order on destroy: project -> wait -> network area region -> network area
resource "time_sleep" "wait_for_project_deletion" {
  destroy_duration = var.project_deletion_wait

  depends_on = [stackit_network_area_region.sna1]
}
