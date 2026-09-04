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
    example    = "alb-private-internal-endpoint"
    managed-by = "terraform"
  }

  # One backend VM per availability zone, keyed by a two-digit index.
  backends      = { for idx, az in var.availability_zones : format("%02d", idx + 1) => az }
  backend_names = { for key, az in local.backends : key => "${var.name_prefix}-backend-${key}" }

  # The backends get fixed addresses. The load balancer needs the target
  # addresses when it is created, and the backend interfaces in turn need the
  # target security group that the load balancer creates, so the addresses
  # cannot be read from the interfaces. The offset leaves the first addresses
  # of the network to the gateway and the DHCP service.
  backend_ips = { for key, az in local.backends : key => cidrhost(var.target_network_cidr, 10 + tonumber(key)) }

  api_host = "api.${var.internal_domain}"

  # The backends serve two HTTPS ports: one with a certificate issued by the
  # private CA of this example, one with a self-signed certificate created on
  # the VM, which the load balancer must refuse.
  backend_port   = 8443
  untrusted_port = 8444

  pool_backends  = "${var.name_prefix}-backends"
  pool_untrusted = "${var.name_prefix}-untrusted"

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
