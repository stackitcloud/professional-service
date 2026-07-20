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

resource "kubernetes_namespace_v1" "alb_demo" {
  metadata {
    name = "alb-demo"
  }
}

resource "kubernetes_ingress_class_v1" "stackit_alb" {
  metadata {
    name = "stackit-alb"
    annotations = {
      # Mandatory; NodePort is currently the only supported network mode.
      "alb.stackit.cloud/network-mode" = "NodePort"
    }
  }

  spec {
    controller = "stackit.cloud/alb-ingress"
  }

  depends_on = [kubernetes_deployment_v1.alb_controller]
}

resource "kubernetes_deployment_v1" "hello" {
  metadata {
    name      = "hello"
    namespace = kubernetes_namespace_v1.alb_demo.metadata[0].name
    labels = {
      app = "hello"
    }
  }

  spec {
    replicas = 2

    selector {
      match_labels = {
        app = "hello"
      }
    }

    template {
      metadata {
        labels = {
          app = "hello"
        }
      }

      spec {
        container {
          name  = "hello"
          image = "nginxdemos/hello:plain-text"

          port {
            container_port = 80
          }
        }
      }
    }
  }
}

resource "kubernetes_service_v1" "hello" {
  metadata {
    name      = "hello"
    namespace = kubernetes_namespace_v1.alb_demo.metadata[0].name
    labels = {
      app = "hello"
    }
  }

  spec {
    type = "NodePort"

    selector = {
      app = "hello"
    }

    port {
      port        = 80
      target_port = 80
      protocol    = "TCP"
    }
  }
}

resource "tls_private_key" "demo" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P256"
}

resource "tls_self_signed_cert" "demo" {
  private_key_pem = tls_private_key.demo.private_key_pem

  subject {
    common_name = var.test_app_hostname
  }

  dns_names             = [var.test_app_hostname]
  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}

resource "kubernetes_secret_v1" "demo_tls" {
  metadata {
    name      = "hello-tls"
    namespace = kubernetes_namespace_v1.alb_demo.metadata[0].name
  }

  type = "kubernetes.io/tls"

  data = {
    "tls.crt" = tls_self_signed_cert.demo.cert_pem
    "tls.key" = tls_private_key.demo.private_key_pem
  }
}

resource "kubernetes_ingress_v1" "hello" {
  metadata {
    name      = "hello"
    namespace = kubernetes_namespace_v1.alb_demo.metadata[0].name
  }

  spec {
    ingress_class_name = kubernetes_ingress_class_v1.stackit_alb.metadata[0].name

    tls {
      secret_name = kubernetes_secret_v1.demo_tls.metadata[0].name
    }

    rule {
      host = var.test_app_hostname

      http {
        path {
          path      = "/"
          path_type = "Prefix"

          backend {
            service {
              name = kubernetes_service_v1.hello.metadata[0].name
              port {
                number = 80
              }
            }
          }
        }
      }
    }
  }

  wait_for_load_balancer = true

  timeouts {
    create = "20m"
  }
}
