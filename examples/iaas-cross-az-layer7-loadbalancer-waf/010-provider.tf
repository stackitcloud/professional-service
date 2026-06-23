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

# Define required providers
terraform {
  required_version = ">= 0.14.0"
  required_providers {
    stackit = {
      source  = "stackitcloud/stackit"
      version = ">= 0.87.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.6.3"
    }
    restapi = {
      source  = "Mastercard/restapi"
      version = ">= 3.0.0"
    }
  }
}

ephemeral "stackit_access_token" "alb" {}

provider "restapi" {
  uri          = "https://alb-waf.api.stackit.cloud"
  bearer_token = ephemeral.stackit_access_token.alb.access_token

  id_attribute         = "name"
  write_returns_object = true
}

provider "stackit" {
  default_region           = var.stackit_region
  service_account_key_path = var.stackit_service_account_key_path
  enable_beta_resources    = true
}
