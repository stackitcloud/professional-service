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

output "cluster_name" {
  description = "Name of the SKE cluster."
  value       = stackit_ske_cluster.this.name
}

output "stackit_service_account_email" {
  description = "Email of the STACKIT service account. Use this when configuring the federated identity provider in the STACKIT Portal."
  value       = stackit_service_account.workload.email
}

output "federation_sub_claim" {
  description = "Value for the sub claim assertion in the federation config. Restricts trust to exactly this Kubernetes ServiceAccount."
  value       = "system:serviceaccount:${var.namespace}:${kubernetes_service_account_v1.workload.metadata[0].name}"
}
