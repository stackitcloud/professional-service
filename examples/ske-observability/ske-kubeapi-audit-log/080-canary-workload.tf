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

# Canary objects, created through the ephemeral kubeconfig.
# Their only purpose is to put a handful of easy-to-find records into the audit
resource "kubernetes_namespace_v1" "audit_canary" {
  metadata {
    name   = "audit-canary"
    labels = local.common_labels
  }
}

resource "kubernetes_config_map_v1" "audit_canary" {
  metadata {
    name      = "audit-canary"
    namespace = kubernetes_namespace_v1.audit_canary.metadata[0].name
    labels    = local.common_labels
  }

  data = {
    purpose = "Generates a create/update audit record on every terraform apply"
  }
}
