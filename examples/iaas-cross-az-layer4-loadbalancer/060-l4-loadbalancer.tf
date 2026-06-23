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

resource "stackit_public_ip" "public_ip" {
  project_id = var.stackit_project_id

  lifecycle {
    ignore_changes = [network_interface_id]
  }
}

resource "stackit_loadbalancer" "this" {
  project_id                        = var.stackit_project_id
  name                              = "lb-example-1"
  disable_security_group_assignment = true

  target_pools = [
    {
      name        = "pool-1"
      target_port = 80
      targets = [
        {
          display_name = "lb-target-1"
          ip           = module.test-machine01.primary_ip
        },
        {
          display_name = "lb-target-2"
          ip           = module.test-machine02.primary_ip
        }
      ]
      active_health_check = {
        healthy_threshold   = 10
        interval            = "3s"
        interval_jitter     = "3s"
        timeout             = "3s"
        unhealthy_threshold = 10
      }
    },
  ]

  listeners = [
    {
      display_name = "listener1"
      port         = 80
      protocol     = "PROTOCOL_TCP"
      target_pool  = "pool-1"
    },
  ]

  networks = [
    {
      network_id = stackit_network.network.network_id
      role       = "ROLE_LISTENERS_AND_TARGETS"
    }
  ]

  external_address = stackit_public_ip.public_ip.ip

  options = {
    // for private loadbalancer usage
    /*private_network_only = false*/
  }
}


output "lb_external_address" {
  value = stackit_loadbalancer.this.external_address
}

/*output "lb_private_ip_address" {
  // for private loadbalancer usage
  value = stackit_loadbalancer.lb_example.private_address
}*/
