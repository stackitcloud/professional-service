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

resource "stackit_kms_keyring" "this" {
  project_id   = var.stackit_project_id
  display_name = "${var.name_prefix}-keyring"
  description  = "Keyring for the Secrets Manager instance ${var.name_prefix}"
}

# The key is the shell, the versions carry the key material. Rotating the key
# adds a version; it does not change which version the instance is pinned to.
resource "stackit_kms_key" "this" {
  project_id   = var.stackit_project_id
  keyring_id   = stackit_kms_keyring.this.keyring_id
  display_name = "${var.name_prefix}-kek"
  description  = "Key encryption key for the Secrets Manager instance ${var.name_prefix}"
  protection   = "software"
  algorithm    = "aes_256_gcm"
  purpose      = "symmetric_encrypt_decrypt"
}
