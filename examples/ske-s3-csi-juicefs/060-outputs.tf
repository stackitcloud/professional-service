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

output "kubeconfig" {
  description = "Kubeconfig for the SKE cluster. Pipe to a file: terraform output -raw kubeconfig > kubeconfig.yaml"
  value       = stackit_ske_kubeconfig.this.kube_config
  sensitive   = true
}

output "juicefs_bucket_name" {
  description = "Name of the STACKIT Object Storage bucket used by JuiceFS for data blocks."
  value       = stackit_objectstorage_bucket.juicefs.name
}

output "juicefs_s3_endpoint" {
  description = "S3-compatible endpoint URL for the JuiceFS bucket (use with AWS CLI --endpoint-url)."
  value       = "https://object.storage.${var.stackit_region}.onstackit.cloud"
}

output "juicefs_s3_access_key" {
  description = "S3 access key for the JuiceFS Object Storage credential."
  value       = stackit_objectstorage_credential.juicefs.access_key
  sensitive   = true
}

output "juicefs_s3_secret_key" {
  description = "S3 secret access key for the JuiceFS Object Storage credential."
  value       = stackit_objectstorage_credential.juicefs.secret_access_key
  sensitive   = true
}

output "juicefs_storage_class" {
  description = "Name of the Kubernetes StorageClass provisioned by the JuiceFS CSI driver."
  value       = kubernetes_storage_class_v1.juicefs.metadata[0].name
}

output "demo_namespace" {
  description = "Kubernetes namespace where the demo workload is deployed."
  value       = kubernetes_namespace_v1.demo.metadata[0].name
}
