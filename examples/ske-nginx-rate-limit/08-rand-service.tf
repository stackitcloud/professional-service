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

resource "random_pet" "suffix" {}

resource "kubernetes_deployment_v1" "random_nginx" {
  metadata {
    name = "nginx-${random_pet.suffix.id}"
  }
  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "nginx-${random_pet.suffix.id}"
      }
    }
    template {
      metadata {
        labels = {
          app = "nginx-${random_pet.suffix.id}"
        }
      }
      spec {
        container {
          name  = "nginx"
          image = "nginx:alpine"
          port {
            container_port = 80
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "nginx" {
  metadata {
    name = "nginx-${random_pet.suffix.id}"
  }
  spec {
    selector = {
      app = "nginx-${random_pet.suffix.id}"
    }
    port {
      port        = 80
      target_port = 80
    }
    type = "ClusterIP"
  }
}

resource "kubernetes_ingress_v1" "nginx" {
  metadata {
    name = "nginx-${random_pet.suffix.id}"
    annotations = {
      "kubernetes.io/ingress.class" = "nginx"
      "nginx.ingress.kubernetes.io/limit-rps" : "10"
    }
  }

  spec {
    rule {
      host = "${stackit_dns_record_set.svc.name}.${stackit_dns_zone.svc_zone.dns_name}"
      http {
        path {
          path      = "/"
          path_type = "Prefix"
          backend {
            service {
              name = kubernetes_service.nginx.metadata[0].name
              port {
                number = 80
              }
            }
          }
        }
      }
    }
  }
}
