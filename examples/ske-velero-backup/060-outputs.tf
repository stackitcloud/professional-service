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
  value       = module.ske.cluster_name
}

output "backup_bucket_name" {
  description = "Name of the Object Storage bucket used for Velero backups."
  value       = stackit_objectstorage_bucket.velero.name
}

output "backup_storage_url" {
  description = "S3-compatible endpoint URL for the Velero backup bucket."
  value       = "https://object.storage.${var.stackit_region}.onstackit.cloud/${stackit_objectstorage_bucket.velero.name}"
}

output "velero_namespace" {
  description = "Kubernetes namespace where Velero is deployed."
  value       = kubernetes_namespace_v1.velero.metadata[0].name
}
