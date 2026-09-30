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

resource "kubernetes_namespace_v1" "collector" {
  metadata {
    name = var.collector_namespace
  }
}

resource "kubernetes_secret_v1" "observability_credentials" {
  metadata {
    name      = "observability-credentials"
    namespace = kubernetes_namespace_v1.collector.metadata[0].name
  }

  data = {
    username = stackit_observability_credential.example.username
    password = stackit_observability_credential.example.password
  }
}

locals {
  gateway_otlp_endpoint = "http://otel-gateway-opentelemetry-collector.${var.collector_namespace}.svc.cluster.local:4318"

  collector_endpoints = {
    secret_name          = kubernetes_secret_v1.observability_credentials.metadata[0].name
    otlp_logs_endpoint   = stackit_observability_instance.example.otlp_http_logs_url
    otlp_traces_endpoint = stackit_observability_instance.example.otlp_http_traces_url
    metrics_push_url     = stackit_observability_instance.example.metrics_push_url
  }
}

# Agent: one Pod per node for container logs and kubelet metrics.
# The "Daemonset Collector" of
# https://opentelemetry.io/docs/platforms/kubernetes/getting-started/#daemonset-collector
resource "helm_release" "otel_agent" {
  name       = "otel-agent"
  repository = "https://open-telemetry.github.io/opentelemetry-helm-charts"
  chart      = "opentelemetry-collector"
  version    = var.otel_chart_version
  namespace  = kubernetes_namespace_v1.collector.metadata[0].name

  values = [
    templatefile("${path.module}/files/otel-agent-values.yaml.tftpl", local.collector_endpoints)
  ]
}

# Gateway: exactly one Pod for cluster state, Kubernetes events and OTLP ingest.
# The "Deployment Collector" of
# https://opentelemetry.io/docs/platforms/kubernetes/getting-started/#deployment-collector
# combined with the gateway pattern of
# https://opentelemetry.io/docs/collector/deployment/gateway/
resource "helm_release" "otel_gateway" {
  name       = "otel-gateway"
  repository = "https://open-telemetry.github.io/opentelemetry-helm-charts"
  chart      = "opentelemetry-collector"
  version    = var.otel_chart_version
  namespace  = kubernetes_namespace_v1.collector.metadata[0].name

  values = [
    templatefile("${path.module}/files/otel-gateway-values.yaml.tftpl", local.collector_endpoints)
  ]
}
