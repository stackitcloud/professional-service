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

# Dedicated public IP so Terraform can wire the DNS A-record in a single apply
resource "stackit_public_ip" "alb" {
  project_id = stackit_resourcemanager_project.showcase.project_id

  labels = {
    environment = "workshop"
    managed-by  = "terraform"
    use-case    = "alb"
  }
}

resource "stackit_application_load_balancer" "workshop" {
  project_id       = stackit_resourcemanager_project.showcase.project_id
  region           = var.stackit_region
  name             = var.alb_name
  plan_id          = var.alb_plan_id
  external_address = stackit_public_ip.alb.ip

  networks = [
    {
      network_id = stackit_network.workshop.network_id
      role       = "ROLE_LISTENERS_AND_TARGETS"
    }
  ]

  listeners = [
    {
      name     = "http"
      port     = 80
      protocol = "PROTOCOL_HTTP"
      http = {
        hosts = [
          {
            host  = "*"
            rules = [{ target_pool = var.alb_target_pool_name }]
          }
        ]
      }
    }
  ]

  target_pools = [
    {
      name        = var.alb_target_pool_name
      target_port = 80
      targets = [
        {
          display_name = "docker-host"
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

  labels = {
    environment = "workshop"
    managed-by  = "terraform"
    use-case    = "alb-certbot"
  }

  # certbot patches the HTTPS listener out-of-band via STACKIT API —
  # ignore_changes prevents Terraform from reverting those updates
  lifecycle {
    ignore_changes = [listeners, target_pools]
  }

  depends_on = [stackit_network_interface.vm]
}
