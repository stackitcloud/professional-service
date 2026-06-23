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

resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = "monitoring"
  }
}

resource "kubernetes_secret" "argus_prometheus_authorization" {
  metadata {
    name      = "argus-prometheus-credentials"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
  }

  data = {
    username = stackit_observability_credential.example.username
    password = stackit_observability_credential.example.password
  }
}

resource "helm_release" "prometheus_operator" {
  name       = "prometheus-operator"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "kube-prometheus-stack"
  version    = "60.1.0"
  namespace  = kubernetes_namespace.monitoring.metadata[0].name

  values = [
    templatefile("prom-values.tftpl", {
      metrics_push_url = stackit_observability_instance.example.metrics_push_url
      secret_name      = kubernetes_secret.argus_prometheus_authorization.metadata[0].name
    })
  ]
}
