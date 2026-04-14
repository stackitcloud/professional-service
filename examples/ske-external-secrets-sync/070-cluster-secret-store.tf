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

resource "kubernetes_secret" "vault_password" {
  depends_on = [helm_release.external_secrets_operator_chart]
  metadata {
    name      = "stackit-secretsmanager-user-password"
    namespace = kubernetes_namespace.external_secrets.metadata.0.name
  }
  data = {
    username = stackit_secretsmanager_user.user.username
    password = stackit_secretsmanager_user.user.password
  }
}

resource "kubernetes_manifest" "stackit_secrets_store" {
  manifest = {
    apiVersion = "external-secrets.io/v1"
    kind       = "ClusterSecretStore"
    metadata = {
      name = "stackit-secrets-store"
    }
    spec = {
      provider = {
        vault = {
          server  = "https://prod.sm.eu01.stackit.cloud"
          path    = stackit_secretsmanager_instance.instance.instance_id
          version = "v2"
          auth = {
            userPass = {
              path     = "userpass"
              username = stackit_secretsmanager_user.user.username
              secretRef = {
                namespace = kubernetes_secret.vault_password.metadata.0.namespace
                name      = kubernetes_secret.vault_password.metadata.0.name
                key       = "password"
              }
            }
          }
        }
      }
    }
  }
}
