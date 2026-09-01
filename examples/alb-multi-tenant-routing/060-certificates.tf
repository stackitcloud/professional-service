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

# One self-signed certificate per hostname. Both are attached to the same
# HTTPS listener; the load balancer presents the one whose name matches the
# SNI of the TLS handshake.
locals {
  certificate_hosts = {
    app   = local.app_host
    admin = local.admin_host
  }
}

resource "tls_private_key" "this" {
  for_each = local.certificate_hosts

  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "this" {
  for_each = local.certificate_hosts

  private_key_pem = tls_private_key.this[each.key].private_key_pem

  subject {
    common_name  = each.value
    organization = "STACKIT Example"
  }

  dns_names             = [each.value]
  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}

resource "stackit_alb_certificate" "this" {
  for_each = local.certificate_hosts

  project_id  = var.stackit_project_id
  region      = var.stackit_region
  name        = "${var.name_prefix}-${each.key}"
  private_key = tls_private_key.this[each.key].private_key_pem
  public_key  = tls_self_signed_cert.this[each.key].cert_pem
}
