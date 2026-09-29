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
  # The features with their defaults (the module derives everything the
  # server gets from them).
  report_enabled = module.cloud_init.features.report.enabled
  delete_enabled = module.cloud_init.features.delete.enabled

  # ---- Where it runs ----

  create_project = var.project_id == null
  project_id     = local.create_project ? stackit_resourcemanager_project.costguard[0].project_id : var.project_id
  create_network = var.network_id == null
  network_id     = local.create_network ? stackit_network.costguard[0].network_id : var.network_id

  # On every resource Terraform creates. do-not-delete keeps costguard (and
  # any other cleanup) away from them; on the project it skips the whole
  # project.
  labels = {
    app             = "costguard"
    "do-not-delete" = "true"
  }

  # ---- The binary ----

  binary = {
    version = var.binary_override != null ? var.binary_override.version : local.release.version
    sha256  = var.binary_override != null ? var.binary_override.sha256 : local.release.sha256
    url     = coalesce(var.download_url, local.release.download_url, "-")
  }
}
