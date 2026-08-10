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
  alb_controller_name = "application-load-balancer-controller"
  alb_controller_labels = {
    "app.kubernetes.io/name"    = "application-load-balancer-controller"
    "app.kubernetes.io/part-of" = "stackit-alb"
  }
}

resource "kubernetes_service_account_v1" "alb_controller" {
  metadata {
    name      = local.alb_controller_name
    namespace = "kube-system"
    labels    = local.alb_controller_labels
  }
}

resource "kubernetes_cluster_role_v1" "alb_controller" {
  metadata {
    name   = local.alb_controller_name
    labels = local.alb_controller_labels
  }

  rule {
    api_groups = ["networking.k8s.io"]
    resources  = ["ingressclasses"]
    verbs      = ["get", "list", "watch", "update", "patch"]
  }

  rule {
    api_groups = ["networking.k8s.io"]
    resources  = ["ingresses"]
    verbs      = ["get", "list", "watch"]
  }
  rule {
    api_groups = ["networking.k8s.io"]
    resources  = ["ingresses/status"]
    verbs      = ["get", "update", "patch"]
  }

  rule {
    api_groups = [""]
    resources  = ["services", "secrets"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = [""]
    resources  = ["nodes"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["events.k8s.io"]
    resources  = ["events"]
    verbs      = ["create", "patch"]
  }
}

resource "kubernetes_cluster_role_binding_v1" "alb_controller" {
  metadata {
    name   = local.alb_controller_name
    labels = local.alb_controller_labels
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role_v1.alb_controller.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account_v1.alb_controller.metadata[0].name
    namespace = "kube-system"
  }
}

resource "kubernetes_role_v1" "alb_controller_leader_election" {
  metadata {
    name      = "${local.alb_controller_name}-leader-election"
    namespace = "kube-system"
    labels    = local.alb_controller_labels
  }

  rule {
    api_groups = ["coordination.k8s.io"]
    resources  = ["leases"]
    verbs      = ["get", "list", "watch", "create", "update", "patch", "delete"]
  }
  rule {
    api_groups = [""]
    resources  = ["events"]
    verbs      = ["create", "patch"]
  }
}

resource "kubernetes_role_binding_v1" "alb_controller_leader_election" {
  metadata {
    name      = "${local.alb_controller_name}-leader-election"
    namespace = "kube-system"
    labels    = local.alb_controller_labels
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = kubernetes_role_v1.alb_controller_leader_election.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account_v1.alb_controller.metadata[0].name
    namespace = "kube-system"
  }
}

resource "kubernetes_config_map_v1" "stackit_cloud_config" {
  metadata {
    name      = "stackit-cloud-config"
    namespace = "kube-system"
    labels    = local.alb_controller_labels
  }

  data = {
    "cloud.yaml" = yamlencode({
      global = {
        projectId = stackit_resourcemanager_project.this.project_id
        region    = var.stackit_region
      }
      applicationLoadBalancer = {
        networkId = stackit_network.ske_nodes.network_id
      }
    })
  }
}

resource "kubernetes_secret_v1" "stackit_cloud_secret" {
  metadata {
    name      = "stackit-cloud-secret"
    namespace = "kube-system"
    labels    = local.alb_controller_labels
  }

  data = {
    "sa_key.json" = stackit_service_account_key.alb_controller.json
  }
}

resource "kubernetes_deployment_v1" "alb_controller" {
  metadata {
    name      = local.alb_controller_name
    namespace = "kube-system"
    labels    = local.alb_controller_labels
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        "app.kubernetes.io/name" = "application-load-balancer-controller"
      }
    }

    template {
      metadata {
        # The gardener.cloud--deny-all NetworkPolicy blocks all traffic of
        # kube-system pods on SKE; DNS, API server and internet egress must
        # be allowed explicitly via these labels.
        labels = merge(local.alb_controller_labels, {
          "networking.gardener.cloud/to-dns"             = "allowed"
          "networking.gardener.cloud/to-apiserver"       = "allowed"
          "networking.gardener.cloud/to-public-networks" = "allowed"
        })
      }

      spec {
        service_account_name             = kubernetes_service_account_v1.alb_controller.metadata[0].name
        termination_grace_period_seconds = 30

        security_context {
          run_as_non_root = true
          run_as_user     = 65532
          run_as_group    = 65532
          fs_group        = 65532
          seccomp_profile {
            type = "RuntimeDefault"
          }
        }

        container {
          name              = "manager"
          image             = var.alb_controller_image
          image_pull_policy = "Always"

          args = [
            "--cloud-config=/etc/config/cloud.yaml",
            "--leader-elect=true",
            "--leader-election-namespace=kube-system",
            "--health-probe-bind-address=:8081",
            "--metrics-bind-address=:8080",
            "--zap-log-level=info",
          ]

          env {
            name  = "STACKIT_SERVICE_ACCOUNT_KEY_PATH"
            value = "/etc/serviceaccount/sa_key.json"
          }

          port {
            name           = "metrics"
            container_port = 8080
            protocol       = "TCP"
          }
          port {
            name           = "probes"
            container_port = 8081
            protocol       = "TCP"
          }

          liveness_probe {
            http_get {
              path = "/healthz"
              port = "probes"
            }
            initial_delay_seconds = 15
            period_seconds        = 20
          }

          readiness_probe {
            http_get {
              path = "/readyz"
              port = "probes"
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "512Mi"
            }
          }

          security_context {
            allow_privilege_escalation = false
            read_only_root_filesystem  = true
            capabilities {
              drop = ["ALL"]
            }
          }

          volume_mount {
            name       = "cloud-config"
            mount_path = "/etc/config"
          }
          volume_mount {
            name       = "cloud-secret"
            mount_path = "/etc/serviceaccount"
          }
        }

        volume {
          name = "cloud-config"
          config_map {
            name = kubernetes_config_map_v1.stackit_cloud_config.metadata[0].name
          }
        }
        volume {
          name = "cloud-secret"
          secret {
            secret_name = kubernetes_secret_v1.stackit_cloud_secret.metadata[0].name
          }
        }
      }
    }
  }

  depends_on = [
    kubernetes_cluster_role_binding_v1.alb_controller,
    kubernetes_role_binding_v1.alb_controller_leader_election,
    stackit_authorization_project_role_assignment.alb_controller,
  ]
}
