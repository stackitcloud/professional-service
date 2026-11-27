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


// Upload the Palo Alto VM-Series image to the project
resource "stackit_image" "paloalto" {
  project_id      = local.project_id
  name            = var.image_name
  local_file_path = pathexpand(var.image_file)
  disk_format     = "qcow2"
  min_disk_size   = 80
  min_ram         = 8192 // MB
  config = {
    uefi = false
  }
}
