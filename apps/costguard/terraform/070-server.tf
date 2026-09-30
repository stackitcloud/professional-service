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


data "stackit_image_v2" "os" {
  project_id = local.project_id
  name       = var.image_name
}

locals {
  image_id = element(split(",", data.stackit_image_v2.os.id), 2)
}

module "cloud_init" {
  source = "./modules/cloud-init"

  settings = {
    organization_id       = var.organization_id
    scope                 = var.scope
    skip                  = var.skip
    regions               = var.regions
    output                = var.output
    warn_empty_after_days = var.warn_empty_after_days
    prices                = var.prices
    features              = var.features
    time_zone             = var.time_zone
    log_level             = var.log_level
  }
  webhook_url           = var.webhook_url
  service_account_email = stackit_service_account.costguard.email
  binary                = local.binary
}

resource "terraform_data" "cloud_init" {
  input = sha256(module.cloud_init.user_data)
}

resource "stackit_server" "costguard" {
  project_id        = local.project_id
  name              = "costguard"
  machine_type      = var.machine_type
  availability_zone = var.availability_zone
  user_data         = module.cloud_init.user_data

  agent = {
    provisioning_policy = "NEVER"
  }

  boot_volume = {
    size                  = var.boot_volume_gb
    source_type           = "image"
    source_id             = local.image_id
    performance_class     = var.boot_volume_performance_class
    delete_on_termination = true
  }

  network_interfaces = [stackit_network_interface.costguard.network_interface_id]
  labels             = local.labels

  lifecycle {
    ignore_changes       = [boot_volume]
    replace_triggered_by = [terraform_data.cloud_init]

    precondition {
      condition     = var.boot_volume_gb >= coalesce(data.stackit_image_v2.os.min_disk_size, 0)
      error_message = "boot_volume_gb is smaller than the image's minimum disk size."
    }
  }

  depends_on = [terraform_data.settings]
}

resource "stackit_server_service_account_attach" "costguard" {
  project_id            = local.project_id
  server_id             = stackit_server.costguard.server_id
  service_account_email = stackit_service_account.costguard.email
}
