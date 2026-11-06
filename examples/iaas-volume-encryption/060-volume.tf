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

data "stackit_image_v2" "this" {
  project_id = var.stackit_project_id
  name_regex = var.image_name_regex
}

resource "stackit_volume" "encrypted" {
  project_id        = var.stackit_project_id
  name              = "${var.name_prefix}-volume"
  availability_zone = var.availability_zone
  performance_class = var.volume_performance_class
  size              = var.volume_size
  labels            = local.labels

  source = {
    type = "image"
    id   = data.stackit_image_v2.this.image_id
  }

  encryption_parameters = {
    kek_keyring_id  = stackit_kms_keyring.volume.keyring_id
    kek_key_id      = stackit_kms_key.volume.key_id
    kek_key_version = var.kek_key_version
    service_account = stackit_service_account.volume_kms.email
  }

  # A newer image must not replace the volume and destroy its data.
  lifecycle {
    ignore_changes = [source]
  }

  depends_on = [stackit_authorization_project_role_assignment.volume_kms]
}
