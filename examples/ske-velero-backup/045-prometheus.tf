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
    name = "monitoring"
  }
}

resource "helm_release" "kube_prometheus_stack" {
  name       = "kube-prometheus-stack"
  namespace  = kubernetes_namespace_v1.monitoring.metadata[0].name
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "kube-prometheus-stack"
  version    = "87.19.1"

  wait    = true
  timeout = 600

  values = [
    yamlencode({
      crds = {
        enabled = true
      }

      prometheus = {
        prometheusSpec = {
          # Scrape ServiceMonitors from all namespaces, not just the release namespace.
          serviceMonitorNamespaceSelector = {}
          serviceMonitorSelector          = {}
        }
      }

      grafana = {
        enabled = true
      }

      alertmanager = {
        enabled = false
      }
    })
  ]
}
