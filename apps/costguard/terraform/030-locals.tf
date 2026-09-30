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
  report_enabled  = module.cloud_init.features.report.enabled
  delete_enabled  = module.cloud_init.features.delete.enabled
  budgets_enabled = module.cloud_init.features.budgets.enabled

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

  binary_version = var.binary_override != null ? var.binary_override.version : local.release.version
  binary_url     = coalesce(var.download_url, local.release.download_url, "-")
  # Semantic version with a v, optionally a pre-release (v0.2.0-rc.1); no
  # build metadata, because "+" would need escaping in the tag and the URL.
  release_version_ok = can(regex("^v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$", local.release.version))

  # The release's SHA256SUMS lines ("<sha256>  costguard_<version>_linux_<arch>"),
  # grouped by architecture; 090-checks.tf requires exactly one per
  # architecture. The file comes from the version's own release, and the
  # server checks the file named with the version against this hash, so the
  # version in the names isn't compared here.
  release_sums = length(data.http.release_sums) == 0 ? {} : {
    for m in regexall("(?m)^([0-9a-f]{64})[ \\t]+\\*?costguard_\\S+_linux_(amd64|arm64)[ \\t]*$", data.http.release_sums[0].response_body) :
    m[1] => m[0]...
  }

  binary = {
    version = local.binary_version
    sha256  = var.binary_override != null ? var.binary_override.sha256 : { for arch, sums in local.release_sums : arch => sums[0] }
    url     = local.binary_url
  }
}

# The hashes of the pinned release, read when Terraform plans: they go into
# the server's user data, so the server installs only the binary this plan
# saw. Not read with binary_override (a test build brings its own hashes).
data "http" "release_sums" {
  # A malformed version is reported by 090-checks.tf instead of a 404.
  count = var.binary_override == null && local.release_version_ok && local.binary_url != "-" ? 1 : 0

  url                = "${trimsuffix(replace(local.binary_url, "{version}", urlencode(local.binary_version)), "/")}/SHA256SUMS"
  request_timeout_ms = 30000

  retry {
    attempts     = 2
    min_delay_ms = 2000
  }

  lifecycle {
    postcondition {
      condition     = self.status_code == 200
      error_message = "costguard ${local.binary_version}: no SHA256SUMS at ${self.url} (HTTP ${self.status_code}). A version can only be installed once its release exists: check out a release tag (apps/costguard/vX.Y.Z), or wait for the release run of the version you pinned."
    }
  }
}
