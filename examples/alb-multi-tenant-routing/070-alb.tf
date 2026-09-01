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

  networks = [
    {
      network_id = stackit_network.this.network_id
      role       = "ROLE_LISTENERS_AND_TARGETS"
    }
  ]

  listeners = [
    {
      name     = "https"
      port     = 443
      protocol = "PROTOCOL_HTTPS"

      https = {
        certificate_config = {
          certificate_ids = [for cert in stackit_alb_certificate.this : cert.cert_id]
        }
      }

      http = {
        hosts = [
          # Hosts are matched on the Host header. Within a host the rules are
          # evaluated in the order they are listed and the first match wins,
          # so specific rules come first and the catch-all comes last. A rule
          # without a path matches every path.
          {
            host = local.app_host
            rules = [
              # A legacy endpoint that still lives in the web application while
              # the rest of /api has moved to the api pool.
              {
                target_pool = local.pool_names["web"]
                path        = { exact_match = "/api/v1/legacy" }
              },
              {
                target_pool = local.pool_names["api"]
                path        = { prefix = "/api" }
              },
              # Canary release of the web application: the same URL, selected by
              # a request header (automated clients) or a query parameter (links
              # handed to testers). Listed after /api, so the API is not affected.
              {
                target_pool = local.pool_names["canary"]
                headers     = [{ name = "X-Canary", exact_match = "true" }]
              },
              {
                target_pool      = local.pool_names["canary"]
                query_parameters = [{ name = "preview", exact_match = "true" }]
              },
              {
                target_pool = local.pool_names["web"]
                path        = { prefix = "/ws" }
                web_socket  = true
              },
              {
                target_pool = local.pool_names["web"]
                path        = { prefix = "/" }
              },
            ]
          },
          {
            host = local.admin_host
            rules = [
              # Stateful application: the load balancer sets a cookie and keeps
              # routing the client to the backend that answered first.
              {
                target_pool = local.pool_names["admin"]
                path        = { prefix = "/" }
                cookie_persistence = {
                  name = "admin-session"
                  ttl  = var.session_cookie_ttl
                }
              },
            ]
          },
        ]
      }
    }
  ]

  target_pools = [
    for name, app in local.applications : {
      name        = local.pool_names[name]
      target_port = app.port
      targets = [
        for key, nic in stackit_network_interface.backend : {
          display_name = "${var.name_prefix}-backend-${key}"
          ip           = nic.ipv4
        }
      ]
      active_health_check = local.active_health_check
    }
  ]

  options = {
    # The load balancer creates and releases its public IP itself.
    ephemeral_address    = true
    private_network_only = false
    access_control = {
      allowed_source_ranges = var.alb_allowed_source_ranges
    }
  }

  # The targets are the addresses of the network interfaces, which exist before
  # the servers do. Wait for the servers so that the load balancer does not
  # start its health checks against ports nobody listens on yet.
  depends_on = [stackit_server.backend]
}
