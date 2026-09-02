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

resource "tls_private_key" "this" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "this" {
  private_key_pem = tls_private_key.this.private_key_pem

  subject {
    common_name  = "${var.name_prefix}.example.internal"
    organization = "STACKIT Example"
  }

  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}

resource "stackit_alb_certificate" "this" {
  project_id  = var.stackit_project_id
  region      = var.stackit_region
  name        = "${var.name_prefix}-certificate"
  private_key = tls_private_key.this.private_key_pem
  public_key  = tls_self_signed_cert.this.cert_pem
}

resource "stackit_public_ip" "alb" {
  project_id = var.stackit_project_id
  labels     = local.labels

  lifecycle {
    ignore_changes = [network_interface_id]
  }
}

resource "stackit_application_load_balancer" "this" {
  project_id       = var.stackit_project_id
  region           = var.stackit_region
  name             = local.alb_name
  plan_id          = var.alb_plan_id
  external_address = stackit_public_ip.alb.ip
  labels           = local.labels

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
      http = {
        hosts = [
          {
            host  = "*"
            rules = [{ target_pool = local.target_pool_name }]
          }
        ]
      }
      https = {
        certificate_config = {
          certificate_ids = [stackit_alb_certificate.this.cert_id]
        }
      }
      waf_config_name = stackit_alb_waf_configuration.this.name
    }
  ]

  target_pools = [
    {
      name        = local.target_pool_name
      target_port = local.backend_port
      targets = [
        for key, nic in stackit_network_interface.backend : {
          display_name = "${var.name_prefix}-backend-${key}"
          ip           = nic.ipv4
        }
      ]
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
  ]

  options = {
    private_network_only = false
    access_control = {
      allowed_source_ranges = var.alb_allowed_source_ranges
    }
    observability = {
      logs = {
        credentials_ref = stackit_loadbalancer_observability_credential.this.credentials_ref
        push_url        = stackit_observability_instance.this.logs_push_url
      }
      metrics = {
        credentials_ref = stackit_loadbalancer_observability_credential.this.credentials_ref
        push_url        = stackit_observability_instance.this.metrics_push_url
      }
    }
  }

  depends_on = [stackit_server.backend]
}
