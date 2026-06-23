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
  project_id   = var.STACKIT_PROJECT_ID
  display_name = "iaas-volume"
  description  = "example description"
}

resource "stackit_kms_key" "volume-key" {
  project_id   = var.STACKIT_PROJECT_ID
  keyring_id   = stackit_kms_keyring.volume.keyring_id
  display_name = "volume-key-01"
  protection   = "software"
  algorithm    = "aes_256_gcm"
  purpose      = "symmetric_encrypt_decrypt"
}
