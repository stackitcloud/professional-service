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

resource "kubernetes_namespace_v1" "example" {
  metadata {
    name = var.workload_namespace
  }
}

resource "kubernetes_config_map_v1" "workload" {
  metadata {
    name      = "logger"
    namespace = kubernetes_namespace_v1.example.metadata[0].name
  }

  data = {
    "app.py" = file("${path.module}/files/app.py")
  }
}

# The application installs Flask and requests, nothing else. Everything
# OpenTelemetry arrives through the annotation below.
resource "kubernetes_deployment_v1" "logger" {
  metadata {
    name      = "logger"
    namespace = kubernetes_namespace_v1.example.metadata[0].name
    labels = {
      app = "logger"
    }
  }

  spec {
    replicas = 2

    selector {
      match_labels = {
        app = "logger"
      }
    }

    template {
      metadata {
        labels = {
          # k8sattributes turns this label into service.name on logs and metrics.
          app = "logger"
        }

        annotations = {
          # This is the whole integration: the operator injects the SDK,
          # the PYTHONPATH and the exporter configuration.
          "instrumentation.opentelemetry.io/inject-python" = "true"
        }
      }

      spec {
        container {
          name  = "logger"
          image = var.workload_image
          command = [
            "sh",
            "-c",
            "pip install --no-cache-dir --quiet flask requests && exec python -u /app/app.py",
          ]

          port {
            container_port = 8080
          }

          volume_mount {
            name       = "workload"
            mount_path = "/app"
            read_only  = true
          }

          env {
            name  = "PIP_ROOT_USER_ACTION"
            value = "ignore"
          }

          resources {
            requests = {
              cpu    = "10m"
              memory = "64Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "256Mi"
            }
          }
        }

        volume {
          name = "workload"

          config_map {
            name = kubernetes_config_map_v1.workload.metadata[0].name
          }
        }
      }
    }
  }

  depends_on = [
    helm_release.otel_gateway,
    kubernetes_manifest.instrumentation,
  ]
}
