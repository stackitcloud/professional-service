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

resource "stackit_image" "custom_image" {
  project_id      = var.project_id
  name            = var.image_name
  disk_format     = var.disk_format
  local_file_path = var.image_file_path

  min_disk_size = var.min_disk_size
  min_ram       = var.min_ram

  labels = var.labels

  config = {
    uefi        = var.uefi
    secure_boot = var.secure_boot
  }
}
