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

# A private CA, created at apply time, issues the certificate of the
# listener and one certificate per backend. The load balancer trusts the CA
# for the connections to the backends (tls_config.custom_ca), the jump host
# trusts it for the connection to the load balancer. All keys are stored in
# the Terraform state; this construction is meant for demonstration.
resource "tls_private_key" "ca" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "ca" {
  private_key_pem   = tls_private_key.ca.private_key_pem
  is_ca_certificate = true

  subject {
    common_name  = "${var.name_prefix} internal CA"
    organization = "STACKIT Example"
  }

  validity_period_hours = 17520

  allowed_uses = [
    "cert_signing",
    "crl_signing",
    "digital_signature",
  ]
}

# Certificate presented by the load balancer to its clients.
resource "tls_private_key" "listener" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "listener" {
  private_key_pem = tls_private_key.listener.private_key_pem

  subject {
    common_name  = local.api_host
    organization = "STACKIT Example"
  }

  dns_names = [local.api_host]
}

resource "tls_locally_signed_cert" "listener" {
  cert_request_pem   = tls_cert_request.listener.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}

resource "stackit_alb_certificate" "listener" {
  project_id  = var.stackit_project_id
  region      = var.stackit_region
  name        = "${var.name_prefix}-listener"
  private_key = tls_private_key.listener.private_key_pem
  public_key  = tls_locally_signed_cert.listener.cert_pem
}

# Certificates presented by the backends to the load balancer. Each carries
# the fixed address of its backend and a name under the internal domain.
resource "tls_private_key" "backend" {
  for_each = local.backends

  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "backend" {
  for_each = local.backends

  private_key_pem = tls_private_key.backend[each.key].private_key_pem

  subject {
    common_name  = "${local.backend_names[each.key]}.${var.internal_domain}"
    organization = "STACKIT Example"
  }

  dns_names    = ["${local.backend_names[each.key]}.${var.internal_domain}"]
  ip_addresses = [local.backend_ips[each.key]]
}

resource "tls_locally_signed_cert" "backend" {
  for_each = local.backends

  cert_request_pem   = tls_cert_request.backend[each.key].cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}
