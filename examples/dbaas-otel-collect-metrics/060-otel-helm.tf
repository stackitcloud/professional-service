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
  sa_json = jsondecode(stackit_service_account_key.this.json)
  otel_helm_values = templatefile("${path.module}/helm-values/otel-collector-values.tftpl", {
    stackit_project_id             = var.stackit_project_id
    stackit_region                 = var.stackit_region
    stackit_postgres_instance_id   = stackit_postgresflex_instance.this.instance_id
    observability_metrics_endpoint = stackit_observability_instance.example.metrics_push_url
    secret_name                    = kubernetes_secret.otel_secret.metadata[0].name
    sa_client_id                   = local.sa_json.credentials.sub
    sa_issuer                      = local.sa_json.credentials.iss
    sa_key_id                      = local.sa_json.credentials.kid
  })
}


resource "stackit_observability_credential" "otel" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.example.instance_id
}

resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = "monitoring"
  }
}

resource "kubernetes_secret" "otel_secret" {
  metadata {
    name      = "otel-secrets"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
  }

  data = {
    OBSERVABILITY_AUTHORIZATION_HEADER = "Basic ${base64encode("${stackit_observability_credential.otel.username}:${stackit_observability_credential.otel.password}")}"
    JSON                               = stackit_service_account_key.this.json
    PRIVATE_KEY                        = jsondecode(stackit_service_account_key.this.json).credentials.privateKey
  }
}

resource "helm_release" "opentelemetry_collector" {
  name       = "opentelemetry-collector"
  repository = "https://open-telemetry.github.io/opentelemetry-helm-charts"
  chart      = "opentelemetry-collector"
  version    = "0.152.0"
  namespace  = kubernetes_namespace.monitoring.metadata[0].name
  timeout    = 30

  values = [
    local.otel_helm_values
  ]
}
