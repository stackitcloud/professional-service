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

locals {
  labels = {
    example    = "alb-multi-tenant-routing"
    managed-by = "terraform"
  }

  # One backend VM per availability zone, keyed by a two-digit index.
  backends = { for idx, az in var.availability_zones : format("%02d", idx + 1) => az }

  # The hostnames the listener routes. Both share the listener and the
  # public IP; the load balancer selects the certificate by SNI.
  app_host   = "app.${var.domain}"
  admin_host = "admin.${var.domain}"

  # One application per target pool. Every backend VM runs all of them, each
  # on its own port, so every pool contains every VM and two machines are
  # enough to make every route highly available.
  applications = {
    web    = { port = 8081 }
    api    = { port = 8082 }
    admin  = { port = 8083 }
    canary = { port = 8084 }
  }

  pool_names = { for name, app in local.applications : name => "${var.name_prefix}-${name}" }

  # Command line of the backend application: one <port>=<pool> pair per
  # application, so that the port decides which pool a response reports.
  backend_arguments = join(" ", [for name, app in local.applications : "${app.port}=${name}"])

  backend_port_min = min([for app in values(local.applications) : app.port]...)
  backend_port_max = max([for app in values(local.applications) : app.port]...)

  # Health check shared by all target pools. The backend answers /healthz
  # with 200 while healthy and 503 after it has been told to fail.
  active_health_check = {
    interval            = "5s"
    interval_jitter     = "1s"
    timeout             = "3s"
    healthy_threshold   = 2
    unhealthy_threshold = 2
    http_health_checks = {
      path      = "/healthz"
      ok_status = ["200"]
    }
  }
}
