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

resource "stackit_public_ip" "alb" {
  project_id = stackit_resourcemanager_project.showcase.project_id

  labels = local.common_labels

  lifecycle {
    ignore_changes = [network_interface_id]
  }
}

resource "stackit_application_load_balancer" "main" {
  project_id       = stackit_resourcemanager_project.showcase.project_id
  region           = var.stackit_region
  name             = "${var.name_prefix}-alb"
  plan_id          = var.alb_plan_id
  external_address = stackit_public_ip.alb.ip

  networks = [
    {
      network_id = stackit_network.main.network_id
      role       = "ROLE_LISTENERS_AND_TARGETS"
    }
  ]

  listeners = [
    {
      name     = "https"
      port     = 443
      protocol = "PROTOCOL_HTTPS"
      http = {
        hosts = [
          {
            host  = "*"
            rules = [{ target_pool = "${var.name_prefix}-pool" }]
          }
        ]
      }
      https = {
        certificate_config = {
          certificate_ids = [stackit_alb_certificate.self_signed.cert_id]
        }
      }
    },
    {
      name     = "http"
      port     = 80
      protocol = "PROTOCOL_HTTP"
      http = {
        hosts = [
          {
            host  = "*"
            rules = [{ target_pool = "${var.name_prefix}-pool" }]
          }
        ]
      }
    }
  ]

  target_pools = [
    {
      name        = "${var.name_prefix}-pool"
      target_port = 80
      targets = [
        {
          display_name = "${var.name_prefix}-vm"
          ip           = stackit_network_interface.vm.ipv4
        }
      ]
    }
  ]

  options = {
    private_network_only = false
    access_control = {
      allowed_source_ranges = var.alb_acl_ranges
    }
  }

  labels = local.common_labels

  depends_on = [stackit_network_interface.vm]
}
