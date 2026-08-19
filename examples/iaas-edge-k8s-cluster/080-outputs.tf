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

output "stec_frontend_url" {
  value       = stackit_edgecloud_instance.this.frontend_url
  description = "STEC management plane web UI URL."
}

output "stec_kubeconfig_path" {
  value       = local_sensitive_file.stec_kubeconfig.filename
  description = "STEC management plane kubeconfig (for EdgeHost/EdgeCluster management)."
}

output "edge_cluster_kubeconfig_path" {
  value       = "${path.module}/.generated/${var.cluster_name}.kubeconfig.yaml"
  description = "Edge cluster kubeconfig."
}

output "edge_cluster_talosconfig_path" {
  value       = "${path.module}/.generated/${var.cluster_name}.talosconfig.yaml"
  description = "Edge cluster talosconfig."
}
