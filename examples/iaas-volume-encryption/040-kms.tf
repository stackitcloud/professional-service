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

resource "stackit_kms_keyring" "volume" {
  project_id   = var.stackit_project_id
  display_name = "${var.name_prefix}-keyring"
  description  = "Keyring for the encrypted block storage volumes of ${var.name_prefix}"
}

resource "stackit_kms_key" "volume" {
  project_id   = var.stackit_project_id
  keyring_id   = stackit_kms_keyring.volume.keyring_id
  display_name = "${var.name_prefix}-key"
  description  = "Key encryption key for the volumes of ${var.name_prefix}"
  protection   = "software"
  algorithm    = "aes_256_gcm"
  purpose      = "symmetric_encrypt_decrypt"
}
