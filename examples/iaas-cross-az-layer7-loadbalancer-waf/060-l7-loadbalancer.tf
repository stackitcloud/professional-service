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

resource "tls_private_key" "example" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "example" {
  private_key_pem = tls_private_key.example.private_key_pem

  subject {
    common_name  = "localhost"
    organization = "STACKIT Test"
  }

  validity_period_hours = 12

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}

resource "stackit_public_ip" "public_ip" {
  project_id = var.stackit_project_id

  lifecycle {
    ignore_changes = [network_interface_id]
  }
}

resource "stackit_alb_certificate" "this" {
  project_id  = var.stackit_project_id
  name        = "example-certificate"
  private_key = tls_private_key.example.private_key_pem
  public_key  = tls_self_signed_cert.example.cert_pem
}

resource "stackit_application_load_balancer" "this" {
  project_id       = var.stackit_project_id
  region           = var.stackit_region
  name             = "example-load-balancer"
  plan_id          = "p10"
  external_address = stackit_public_ip.public_ip.ip

  listeners = [
    {
      name = "listener01"
      port = 443
      http = {
        hosts = [{
          host = "*"
          rules = [{
            target_pool = "target-pool-01"
            /*path = {
              prefix = "/path"
            }*/
          }]
        }]
      }
      https = {
        certificate_config = {
          certificate_ids = [
            stackit_alb_certificate.this.cert_id
          ]
        }
      }
      waf_config_name = restapi_object.waf.api_data.name
      protocol        = "PROTOCOL_HTTPS"
    }
  ]
  networks = [
    {
      network_id = stackit_network.network.network_id
      role       = "ROLE_LISTENERS_AND_TARGETS"
    }
  ]
  target_pools = [
    {
      name        = "target-pool-01"
      target_port = 80
      targets = [
        {
          display_name = "server01"
          ip           = module.test-machine01.primary_ip
        },
        {
          display_name = "server02"
          ip           = module.test-machine02.primary_ip
        }
      ]
    }
  ]
}


output "alb_external_address" {
  value = stackit_application_load_balancer.this.external_address
}

/*output "alb_private_ip_address" {
  // for private alb loadbalancer usage
  value = stackit_application_load_balancer.this.private_address
}*/
