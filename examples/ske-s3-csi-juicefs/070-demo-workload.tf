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

resource "kubernetes_namespace_v1" "demo" {
  metadata {
    name = "juicefs-demo"
    labels = {
      "app.kubernetes.io/managed-by" = "terraform"
    }
  }

  depends_on = [helm_release.juicefs_csi]
}

# Single PVC shared across all replicas. ReadWriteMany is what JuiceFS enables.
resource "kubernetes_persistent_volume_claim_v1" "demo" {
  metadata {
    name      = "juicefs-pvc"
    namespace = kubernetes_namespace_v1.demo.metadata[0].name
  }

  spec {
    access_modes       = ["ReadWriteMany"]
    storage_class_name = kubernetes_storage_class_v1.juicefs.metadata[0].name

    resources {
      requests = {
        storage = "10Gi"
      }
    }
  }

  wait_until_bound = false
}

# Two replicas write to /data/shared.txt on the same PVC, proving RWX works.
resource "kubernetes_deployment_v1" "demo" {
  metadata {
    name      = "juicefs-demo"
    namespace = kubernetes_namespace_v1.demo.metadata[0].name
  }

  spec {
    replicas = 2

    selector {
      match_labels = {
        app = "juicefs-demo"
      }
    }

    template {
      metadata {
        labels = {
          app = "juicefs-demo"
        }
      }

      spec {
        container {
          name  = "writer"
          image = "busybox:1.36"

          command = [
            "/bin/sh", "-c",
            "while true; do echo \"$(hostname): $(date)\" >> /data/shared.txt; sleep 3; done",
          ]

          volume_mount {
            name       = "juicefs-vol"
            mount_path = "/data"
          }

          resources {
            requests = {
              cpu    = "50m"
              memory = "32Mi"
            }
            limits = {
              memory = "64Mi"
            }
          }
        }

        volume {
          name = "juicefs-vol"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim_v1.demo.metadata[0].name
          }
        }
      }
    }
  }

  depends_on = [kubernetes_storage_class_v1.juicefs]
}
