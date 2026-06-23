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
  gpu_operator_helm_values = templatefile("${path.module}/gpu-operator-values.yaml.tftpl", {})
}

resource "kubernetes_namespace_v1" "gpu_operator" {
  metadata {
    name = "gpu-operator"
  }
}

resource "helm_release" "gpu_operator" {
  name       = "gpu-operator"
  namespace  = kubernetes_namespace_v1.gpu_operator.metadata[0].name
  repository = "https://helm.ngc.nvidia.com/nvidia"
  chart      = "gpu-operator"
  version    = "25.3.1"

  values = [
    local.gpu_operator_helm_values
  ]
}
