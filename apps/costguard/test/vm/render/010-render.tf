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


terraform {
  required_version = ">= 1.9"
}

variable "webhook_url" {
  description = "The fake webhook (served by fake_services.py)."
  type        = string
  sensitive   = true
}

variable "download_url" {
  description = "The fake download server (serves dist/)."
  type        = string
}

variable "binary_version" {
  description = "The version make dist built."
  type        = string
}

variable "binary_sha256" {
  description = "The pinned hash (the real one, or a wrong one for the mismatch test)."
  type        = string
}

variable "features" {
  description = "Same shape as the root config's features."
  type        = any
}

module "cloud_init" {
  source = "../../../terraform/modules/cloud-init"

  settings = {
    organization_id       = "11111111-0000-4000-8000-000000000000"
    scope                 = {}
    skip                  = { projects = ["99999999-0000-4000-8000-000000000001"] }
    regions               = ["eu01"]
    output                = "slack"
    warn_empty_after_days = 30
    features              = var.features
    time_zone             = "Europe/Berlin"
    log_level             = "debug"
  }
  webhook_url           = var.webhook_url
  service_account_email = "costguard-vmtest@sa.stackit.cloud"
  binary = {
    version = var.binary_version
    url     = var.download_url
    sha256  = { amd64 = var.binary_sha256 }
  }
}

output "user_data" {
  description = "The user data, readable (only fake secrets in here)."
  value       = nonsensitive(module.cloud_init.user_data)
}
