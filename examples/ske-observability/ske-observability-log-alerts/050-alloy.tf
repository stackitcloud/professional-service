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

resource "kubernetes_namespace_v1" "monitoring" {
  metadata {
    name = var.monitoring_namespace
  }
}

# The credentials stay in a Secret and reach Alloy as environment variables, so
# they never end up in the Helm values or in the rendered Alloy configuration.
resource "kubernetes_secret_v1" "observability_credentials" {
  metadata {
    name      = "observability-credentials"
    namespace = kubernetes_namespace_v1.monitoring.metadata[0].name
  }

  data = {
    username = stackit_observability_credential.example.username
    password = stackit_observability_credential.example.password
  }
}

resource "helm_release" "alloy" {
  name       = "alloy"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "alloy"
  version    = var.alloy_chart_version
  namespace  = kubernetes_namespace_v1.monitoring.metadata[0].name

  values = [yamlencode({
    # The chart only ships the PodLogs CRD, which loki.source.podlogs needs.
    # This example discovers Pods directly, so the CRD is not required.
    crds = {
      create = false
    }

    controller = {
      type = "daemonset"
    }

    alloy = {
      # Mounts the host's /var/log, where the kubelet writes container logs.
      mounts = {
        varlog = true
      }

      extraEnv = [
        {
          name = "NODE_NAME"
          valueFrom = {
            fieldRef = {
              fieldPath = "spec.nodeName"
            }
          }
        },
        {
          name = "OBSERVABILITY_USERNAME"
          valueFrom = {
            secretKeyRef = {
              name = kubernetes_secret_v1.observability_credentials.metadata[0].name
              key  = "username"
            }
          }
        },
        {
          name = "OBSERVABILITY_PASSWORD"
          valueFrom = {
            secretKeyRef = {
              name = kubernetes_secret_v1.observability_credentials.metadata[0].name
              key  = "password"
            }
          }
        },
      ]

      configMap = {
        content = templatefile("${path.module}/alloy-config.alloy.tftpl", {
          logs_push_url = stackit_observability_instance.example.logs_push_url
        })
      }
    }
  })]
}
