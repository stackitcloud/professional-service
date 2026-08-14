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
  # Image name encodes the Talos version so a version bump produces a new image.
  iaas_image_name = "talos-${var.talos_version}"
}

# Applies the EdgeImage CRD to STEC, waits for it to be ready, downloads
# the raw artifact, uploads it to STACKIT IaaS, and writes
# .generated/image-ids.json with the resulting image ID.
#
# stackit_image cannot be used here because its local_file_path is validated
# at plan time — before the image exists. The stackit CLI upload + data "external"
# defers everything to apply time.
resource "null_resource" "talos_image" {
  triggers = {
    talos_version = var.talos_version
    disk_size_gb  = var.disk_size_gb
  }

  provisioner "local-exec" {
    working_dir = path.module
    command     = "bash scripts/01-create-edge-image.sh"
    environment = {
      KUBECONFIG         = local_sensitive_file.stec_kubeconfig.filename
      STACKIT_PROJECT_ID = var.stackit_project_id
      STACKIT_REGION     = var.stackit_region
      TALOS_VERSION      = var.talos_version
      IMAGE_NAME         = local.iaas_image_name
      DOWNLOAD_DIR       = "${path.module}/downloads"
      OUTPUT_DIR         = "${path.module}/.generated"
      DISK_SIZE_GB       = var.disk_size_gb
    }
  }

  depends_on = [local_sensitive_file.stec_kubeconfig]
}

# Reads the image ID written by the script. Depends on a managed resource so
# Terraform defers evaluation to apply time.
data "external" "image_ids" {
  program    = ["bash", "-c", "cat '${path.module}/.generated/image-ids.json'"]
  depends_on = [null_resource.talos_image]
}
