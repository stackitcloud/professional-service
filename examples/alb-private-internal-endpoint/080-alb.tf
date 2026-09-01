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

resource "stackit_application_load_balancer" "this" {
  project_id = var.stackit_project_id
  region     = var.stackit_region
  name       = "${var.name_prefix}-alb"
  plan_id    = var.alb_plan_id
  labels     = local.labels

  # The targets are not in the network of the load balancer, so the load
  # balancer must not assign its target security group itself; 060-backends.tf
  # assigns the exported group to the backend interfaces. Not changeable after
  # creation.
  disable_target_security_group_assignment = true

  # The load balancer lives in the listener network only; the targets in the
  # target network are reached through the project router. Declaring the
  # target network as a second entry with the roles ROLE_LISTENERS and
  # ROLE_TARGETS is accepted by the API, but the listener never answered on
  # the private address in that configuration (see README). Not changeable
  # after creation.
  networks = [
    {
      network_id = stackit_network.listener.network_id
      role       = "ROLE_LISTENERS_AND_TARGETS"
    },
  ]

  listeners = [
    {
      name     = "https"
      port     = 443
      protocol = "PROTOCOL_HTTPS"

      https = {
        certificate_config = {
          certificate_ids = [stackit_alb_certificate.listener.cert_id]
        }
      }

      http = {
        hosts = [
          {
            host = local.api_host
            rules = [
              # Pool whose backends present a certificate the load balancer does
              # not trust; requests to it fail and show that validation is on.
              {
                target_pool = local.pool_untrusted
                path        = { prefix = "/untrusted" }
              },
              {
                target_pool = local.pool_backends
                path        = { prefix = "/" }
              },
            ]
          },
        ]
      }
    }
  ]

  # TLS bridging: the load balancer terminates the client connection and opens
  # a new TLS connection to the backend, whose certificate must chain to the
  # private CA. Both pools use the same configuration; they differ only in the
  # certificate the backends present on the target port.
  target_pools = [
    for pool in [
      { name = local.pool_backends, port = local.backend_port },
      { name = local.pool_untrusted, port = local.untrusted_port },
      ] : {
      name        = pool.name
      target_port = pool.port
      targets = [
        for key, ip in local.backend_ips : {
          display_name = local.backend_names[key]
          ip           = ip
        }
      ]
      active_health_check = local.active_health_check
      tls_config = {
        enabled                     = true
        skip_certificate_validation = false
        custom_ca                   = tls_self_signed_cert.ca.cert_pem
      }
    }
  ]

  options = {
    # No public address at all; the load balancer is reachable through its
    # private address only. Not changeable after creation.
    private_network_only = true
  }
}
