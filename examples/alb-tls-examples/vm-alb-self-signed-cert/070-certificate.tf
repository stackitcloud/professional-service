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

resource "tls_private_key" "self_signed" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "self_signed" {
  private_key_pem = tls_private_key.self_signed.private_key_pem

  subject {
    common_name  = var.tls_common_name
    organization = var.tls_organization
  }

  dns_names             = [var.tls_common_name]
  validity_period_hours = var.tls_validity_hours

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}

resource "stackit_alb_certificate" "self_signed" {
  project_id  = stackit_resourcemanager_project.showcase.project_id
  region      = var.stackit_region
  name        = "${var.name_prefix}-selfsigned-cert"
  public_key  = tls_self_signed_cert.self_signed.cert_pem
  private_key = tls_private_key.self_signed.private_key_pem
}
