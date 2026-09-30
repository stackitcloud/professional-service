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

# The operator brings the Instrumentation CRD and the admission webhook that
# injects the SDK into annotated Pods.
# This is only needed for collecting traces

resource "helm_release" "otel_operator" {
  name             = "opentelemetry-operator"
  repository       = "https://open-telemetry.github.io/opentelemetry-helm-charts"
  chart            = "opentelemetry-operator"
  version          = var.otel_operator_chart_version
  namespace        = var.operator_namespace
  create_namespace = true

  values = [yamlencode({
    manager = {
      collectorImage = {
        repository = "otel/opentelemetry-collector-contrib"
      }
    }

    admissionWebhooks = {
      # Helm generates the webhook certificate, so the example needs no
      # cert-manager in the cluster.
      certManager = {
        enabled = false
      }
      autoGenerateCert = {
        enabled = true
      }
    }
  })]
}

# Pods carrying the annotation instrumentation.opentelemetry.io/inject-python
# get this configuration. The SDK arrives through an init container, so the
# application image stays untouched.
resource "kubernetes_manifest" "instrumentation" {
  manifest = {
    apiVersion = "opentelemetry.io/v1alpha1"
    kind       = "Instrumentation"
    metadata = {
      name      = "python"
      namespace = kubernetes_namespace_v1.example.metadata[0].name
    }
    spec = {
      exporter = {
        endpoint = local.gateway_otlp_endpoint
      }

      propagators = ["tracecontext", "baggage"]

      sampler = {
        type     = "parentbased_traceidratio"
        argument = "1"
      }

      python = {
        env = [
          {
            name  = "OTEL_EXPORTER_OTLP_PROTOCOL"
            value = "http/protobuf"
          },
          {
            name  = "OTEL_PYTHON_LOG_CORRELATION"
            value = "true"
          },
          {
            name  = "OTEL_METRICS_EXPORTER"
            value = "none"
          },
          {
            name  = "OTEL_LOGS_EXPORTER"
            value = "none"
          },
        ]
      }
    }
  }

  depends_on = [helm_release.otel_operator]
}
