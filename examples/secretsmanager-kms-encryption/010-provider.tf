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
  required_version = ">= 1.0.0"
  required_providers {
    stackit = {
      source  = "stackitcloud/stackit"
      version = ">= 0.113.0"
    }
    vault = {
      source  = "hashicorp/vault"
      version = ">= 5.9.0"
    }
  }
}

locals {
  # Base URL of the Secrets Manager Vault API (KV v2 compatible).
  sm_vault_address = "https://prod.sm.eu01.stackit.cloud"
  # Table the round-trip script creates in the database and verifies.
  roundtrip_table = "sm_kms_roundtrip"
}

# The service account key file must contain the
# credentials.privateKey field, e.g. a key created with
# `stackit service-account key create`.
provider "stackit" {
  default_region           = var.stackit_region
  service_account_key_path = var.stackit_service_account_key_path
  experiments              = ["iam"]
}

# The Secrets Manager instance speaks the Vault KV v2 API. The provider
# logs in with the user created below and writes the database credentials
# into the instance.
provider "vault" {
  address          = local.sm_vault_address
  skip_child_token = true

  auth_login_userpass {
    username = stackit_secretsmanager_user.this.username
    password = stackit_secretsmanager_user.this.password
  }
}
