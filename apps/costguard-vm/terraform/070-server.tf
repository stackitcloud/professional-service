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
  # image_id is Optional but not Computed in stackit_image_v2: while the data
  # source waits for a new project it is null in the plan, and the server
  # would get source_id = null. The computed id ("project_id,region,image_id")
  # is unknown instead, which is what the plan needs.
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
  break_glass           = var.break_glass != null
}

# A second line of defence: user_data already forces a replacement, this
# also covers a provider that stops doing so.
resource "terraform_data" "cloud_init" {
  input = sha256(module.cloud_init.user_data)
}

# Immutable: any settings change replaces the server (costguard-vm keeps no
# state). The image only refreshes with a replacement; STACKIT rebuilds it
# daily, so following it would replace the server on every apply. For a
# fresh image: terraform apply -replace=stackit_server.costguard
resource "stackit_server" "costguard" {
  project_id        = local.project_id
  name              = "costguard"
  machine_type      = var.machine_type
  availability_zone = var.availability_zone
  user_data         = module.cloud_init.user_data
  keypair_name      = var.break_glass == null ? null : stackit_key_pair.break_glass[0].name

  # No STACKIT Server Agent: it runs scripts sent through the Run Command API
  # as root, which could read the service account's token.
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

# The provider can't attach a service account at creation (the IaaS API
# could), so this follows right after; the boot run waits for it.
resource "stackit_server_service_account_attach" "costguard" {
  project_id            = local.project_id
  server_id             = stackit_server.costguard.server_id
  service_account_email = stackit_service_account.costguard.email
}

resource "stackit_key_pair" "break_glass" {
  count = var.break_glass == null ? 0 : 1

  name       = "costguard-break-glass"
  public_key = var.break_glass.public_key
  labels     = local.labels
}
