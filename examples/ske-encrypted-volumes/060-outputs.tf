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

output "ske_cluster_name" {
  description = "Name of the SKE cluster"
  value       = stackit_ske_cluster.default.name
}

output "kubeconfig_command" {
  description = "Fetch a kubeconfig for kubectl"
  value       = "stackit ske kubeconfig create ${stackit_ske_cluster.default.name} --project-id ${var.stackit_project_id} --region ${stackit_ske_cluster.default.region} --expiration 8h"
}

output "storage_class_name" {
  description = "Encrypted StorageClass to reference from a PersistentVolumeClaim"
  value       = kubernetes_storage_class_v1.encrypted_premium.metadata[0].name
}
