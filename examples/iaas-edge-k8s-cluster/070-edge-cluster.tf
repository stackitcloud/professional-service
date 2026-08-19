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

# EdgeCluster is a CRD on the STEC Kubernetes API — not managed by the
# STACKIT Terraform provider. The script waits for all VMs to register as
# EdgeHosts (matched by STACKIT server UUID), applies the EdgeCluster manifest
# with cloud proxy enabled, and extracts credentials once the cluster is ready.
resource "null_resource" "edge_cluster" {
  triggers = {
    cp_server_ids      = join(",", stackit_server.cp[*].server_id)
    worker_server_ids  = join(",", stackit_server.worker[*].server_id)
    cluster_name       = var.cluster_name
    talos_version      = var.talos_version
    kubernetes_version = var.kubernetes_version
  }

  provisioner "local-exec" {
    working_dir = path.module
    command     = "bash scripts/02-create-cluster.sh"
    environment = {
      KUBECONFIG         = local_sensitive_file.stec_kubeconfig.filename
      CLUSTER_NAME       = var.cluster_name
      TALOS_VERSION      = var.talos_version
      KUBERNETES_VERSION = var.kubernetes_version
      OUTPUT_DIR         = "${path.module}/.generated"
      CP_SERVER_IDS      = join(",", stackit_server.cp[*].server_id)
      WORKER_SERVER_IDS  = join(",", stackit_server.worker[*].server_id)
    }
  }

  depends_on = [
    stackit_server.cp,
    stackit_server.worker,
  ]
}
