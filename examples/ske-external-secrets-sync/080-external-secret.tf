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

resource "kubernetes_namespace" "dev" {
  metadata {
    name = "dev"
  }
}

resource "kubernetes_manifest" "random_secret_sync" {
  manifest = {
    apiVersion = "external-secrets.io/v1"
    kind       = "ExternalSecret"
    metadata = {
      name      = "random-secret-sync"
      namespace = kubernetes_namespace.dev.metadata.0.name
    }
    spec = {
      refreshInterval = "30s"
      secretStoreRef = {
        name = "stackit-secrets-store"
        kind = "ClusterSecretStore"
      }
      target = {
        name           = "random-secret-sync"
        creationPolicy = "Owner"
      }
      data = [
        {
          secretKey = "admin"
          remoteRef = {
            key      = "random-secret"
            property = "admin"
          }
        }
      ]
    }
  }
  depends_on = [helm_release.external_secrets_operator_chart]
}
