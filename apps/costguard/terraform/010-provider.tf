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
  # >= 1.9: variable validations may refer to other variables.
  required_version = ">= 1.9"

  required_providers {
    stackit = {
      source = "stackitcloud/stackit"
      # 0.115.0 fixed "server recreated on update when agent is unconfigured".
      # .terraform.lock.hcl pins the tested version; upgrade it only after a
      # plan check (the role assignments are an experimental feature).
      version = ">= 0.115.0"
    }
    # Reads the release's SHA256SUMS when Terraform plans (030-locals.tf).
    http = {
      source  = "hashicorp/http"
      version = ">= 3.6.0"
    }
  }
}

# The deployer's key file from terraform.tfvars; without it the provider
# falls back to STACKIT_SERVICE_ACCOUNT_KEY_PATH or ~/.stackit/credentials.json.
provider "stackit" {
  default_region           = var.region
  service_account_key_path = var.service_account_key_path == null ? null : pathexpand(var.service_account_key_path)
  # The role assignment resources are behind the provider's "iam" experiment
  # ("unstable features without official support"); the landing zone does the
  # same. The whole install depends on it.
  experiments = ["iam"]
  # stackit_image_v2 (the only image lookup by name) is a beta data source.
  enable_beta_resources = true
}
