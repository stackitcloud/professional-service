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

variable "settings" {
  description = "costguard's settings, as the root config's variables of the same names."
  type = object({
    organization_id       = string
    scope                 = map(list(string))
    skip                  = map(list(string))
    regions               = list(string)
    output                = string
    warn_empty_after_days = number
    features              = any
    time_zone             = string
    log_level             = string
  })
}

variable "webhook_url" {
  description = "The chat webhook; ends up in the 0600 environment file."
  type        = string
  sensitive   = true
}

variable "service_account_email" {
  description = "The service account attached to the server; costguard asks the metadata service for its token."
  type        = string
}

variable "binary" {
  description = "What the install unit downloads: version, download URL ({version} is filled in) and the pinned SHA-256 per architecture."
  type = object({
    version = string
    url     = string
    sha256  = map(string)
  })
}
