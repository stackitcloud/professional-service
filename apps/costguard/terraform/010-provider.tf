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

  required_providers {
    stackit = {
      source  = "stackitcloud/stackit"
      version = ">= 0.115.0"
    }
    http = {
      source  = "hashicorp/http"
      version = ">= 3.6.0"
    }
  }
}

provider "stackit" {
  default_region           = var.region
  service_account_key_path = var.service_account_key_path == null ? null : pathexpand(var.service_account_key_path)
  experiments              = ["iam"]
  enable_beta_resources    = true
}
