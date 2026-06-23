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

output "cdn_distribution_id" {
  description = "The STACKIT CDN distribution ID"
  value       = stackit_cdn_distribution.website.distribution_id
}

output "cdn_managed_domain" {
  description = "The managed CDN domain for the distribution"
  value       = stackit_cdn_distribution.website.domains[0].name
}

output "cdn_status" {
  description = "The current status of the CDN distribution"
  value       = stackit_cdn_distribution.website.status
}

output "bucket_name" {
  description = "The STACKIT Object Storage bucket name"
  value       = stackit_objectstorage_bucket.website.name
}
