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

output "image_id" {
  description = "ID of the uploaded custom image — use this UUID when creating servers or volumes"
  value       = stackit_image.custom_image.image_id
}

output "image_name" {
  description = "Name of the uploaded custom image"
  value       = stackit_image.custom_image.name
}

output "image_scope" {
  description = "Scope of the image (private or public)"
  value       = stackit_image.custom_image.scope
}

output "checksum_algorithm" {
  description = "Algorithm used for the image checksum"
  value       = stackit_image.custom_image.checksum.algorithm
}

output "checksum_digest" {
  description = "Checksum digest of the uploaded image — verify this against your local file"
  value       = stackit_image.custom_image.checksum.digest
}

output "server_id" {
  description = "ID of the server created from the custom image"
  value       = stackit_server.from_custom_image.server_id
}
