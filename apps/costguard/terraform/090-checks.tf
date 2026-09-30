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


# Rules across several settings. As preconditions they stop the plan before
# anything is created (the server depends on this resource).
resource "terraform_data" "settings" {
  lifecycle {
    precondition {
      condition     = !local.delete_enabled || local.report_enabled
      error_message = "features.delete needs features.report: nothing is deleted without a report message first."
    }

    precondition {
      condition     = local.report_enabled || local.budgets_enabled
      error_message = "No feature is enabled, so costguard would never post anything: enable features.report or features.budgets."
    }

    precondition {
      condition     = !local.delete_enabled || !local.report_enabled || min(concat(module.cloud_init.delete_gaps_hours, [168])...) >= 20
      error_message = "Every delete run must come at least 20 hours after the report run before it (that run labels what gets deleted and announces it). Now: ${join(", ", [for h in module.cloud_init.delete_gaps_hours : "${h} h"])}."
    }

    precondition {
      condition     = local.binary.version != ""
      error_message = "This checkout pins no costguard release (000-release.tf is empty; the first release pins it). Check out a release tag (apps/costguard/vX.Y.Z), or set binary_override for a test build."
    }

    precondition {
      condition     = var.binary_override != null || local.release.version == "" || local.release_version_ok
      error_message = "000-release.tf: version must be vMAJOR.MINOR.PATCH, optionally with -pre.release (e.g. v0.2.0 or v0.2.0-rc.1); got \"${local.release.version}\"."
    }

    precondition {
      condition     = var.binary_override != null || !local.release_version_ok || alltrue([for arch in ["amd64", "arm64"] : length(try(local.release_sums[arch], [])) == 1])
      error_message = "The SHA256SUMS of costguard ${local.binary.version} must list costguard_${local.binary.version}_linux_amd64 and _arm64 once each (found: ${jsonencode(local.release_sums)})."
    }

    precondition {
      condition     = local.binary.url != "-"
      error_message = "download_url is required: this checkout has no release location (000-release.tf)."
    }
  }
}

# Warnings: they show in plan and apply but don't stop anything.

check "project_parent" {
  assert {
    condition     = !local.create_project || var.parent_container_id == null || var.parent_container_id == var.organization_id
    error_message = "The costguard project is created under a folder. Anyone who is owner, editor or Service Account User on that folder can act as costguard's service account (and so use its organization-wide rights). Prefer the organization as parent."
  }
}

check "webhook_matches_output" {
  assert {
    condition = nonsensitive(anytrue([
      var.output == "slack" && can(regex("^https://hooks\\.slack\\.com/", var.webhook_url)),
      var.output == "googlechat" && can(regex("^https://chat\\.googleapis\\.com/", var.webhook_url)),
      var.output == "teams" && can(regex("^https://[^/]+\\.(logic\\.azure\\.com|powerplatform\\.com|webhook\\.office\\.com)(:443)?/", var.webhook_url)),
    ]))
    error_message = "The webhook URL does not look like a ${var.output} webhook. Check output and TF_VAR_webhook_url: messages in the wrong format are rejected by the chat."
  }
}

check "reboot_window" {
  assert {
    condition = alltrue([for name, time in module.cloud_init.run_times :
      tonumber(replace(time, ":", "")) < 330 || tonumber(replace(time, ":", "")) > 430
    ])
    error_message = "A run is scheduled between 03:30 and 04:30. unattended-upgrades reboots the server at 04:00 after kernel updates, which stops a running job."
  }
}
