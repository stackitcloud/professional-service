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


# Offline tests of the root config: terraform test (no STACKIT access; the
# provider is mocked). Covers the variable checks, the rules across settings,
# what the server gets (config, units, environment) and which resources each
# option creates.

mock_provider "stackit" {
  mock_resource "stackit_resourcemanager_project" {
    defaults = { project_id = "99999999-0000-4000-8000-000000000001" }
  }
  mock_resource "stackit_service_account" {
    defaults = { email = "costguard-ab12cd@sa.stackit.cloud" }
  }
  mock_resource "stackit_network" {
    defaults = { network_id = "99999999-0000-4000-8000-000000000002" }
  }
  mock_resource "stackit_security_group" {
    defaults = { security_group_id = "99999999-0000-4000-8000-000000000003" }
  }
  mock_resource "stackit_network_interface" {
    defaults = { network_interface_id = "99999999-0000-4000-8000-000000000004" }
  }
  mock_resource "stackit_server" {
    defaults = { server_id = "99999999-0000-4000-8000-000000000005" }
  }
  mock_resource "stackit_public_ip" {
    defaults = { ip = "192.0.2.10" }
  }
  mock_data "stackit_image_v2" {
    defaults = {
      id            = "99999999-0000-4000-8000-000000000001,eu01,99999999-0000-4000-8000-000000000006"
      min_disk_size = 3
    }
  }
}

# Every variable is set here: terraform test also reads a local
# terraform.tfvars, and the tests must not depend on anyone's settings.
variables {
  service_account_key_path      = null
  organization_id               = "11111111-0000-4000-8000-000000000000"
  project_id                    = null
  parent_container_id           = null
  project_owner_email           = "deployer@sa.stackit.cloud"
  network_id                    = null
  network_ipv4_prefix           = "10.64.0.0/28"
  dns_nameservers               = ["9.9.9.9"]
  region                        = "eu01"
  availability_zone             = "eu01-1"
  machine_type                  = "t2i.1"
  image_name                    = "Debian 13"
  boot_volume_gb                = 10
  boot_volume_performance_class = "storage_premium_perf0"
  time_zone                     = "Europe/Berlin"
  break_glass                   = null
  output                        = "slack"
  webhook_url                   = "https://hooks.slack.com/services/T0/B0/secret"
  features                      = {}
  scope                         = {}
  skip                          = {}
  regions                       = ["eu01"]
  warn_empty_after_days         = 30
  prices                        = {}
  log_level                     = "info"
  binary_override = {
    version = "v0.1.0-test"
    sha256  = { amd64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" }
  }
  download_url = "https://downloads.example/{version}/"
}

# ---- Defaults: report only ----

run "report_only_by_default" {
  command = apply

  assert {
    condition     = module.cloud_init.config.deleteEnabled == false && !contains(keys(module.cloud_init.config), "deleteRunAt")
    error_message = "With delete off, the config must say deleteEnabled: false and have no deleteRunAt."
  }
  assert {
    condition     = module.cloud_init.config.reportRunAt == "Monday 08:00 (Europe/Berlin)"
    error_message = "reportRunAt: ${module.cloud_init.config.reportRunAt}"
  }
  assert {
    condition     = length(module.cloud_init.timers) == 1 && module.cloud_init.timers[0].subcommand == "report" && module.cloud_init.timers[0].on_calendar == "Mon *-*-* 08:00:00 Europe/Berlin"
    error_message = "Expected one report timer running `report` on Mondays at 08:00."
  }
  assert {
    condition     = length(stackit_authorization_organization_custom_role.cleaner) == 0 && length(stackit_authorization_organization_role_assignment.cleaner) == 0
    error_message = "Without delete there must be no cleaner role."
  }
  assert {
    condition     = stackit_authorization_organization_role_assignment.reader.role == "costguard.reader" && stackit_authorization_organization_role_assignment.reader.resource_id == var.organization_id
    error_message = "costguard.reader must be assigned on the organization (the binary's hint names it)."
  }
  assert {
    condition     = length(stackit_authorization_organization_custom_role.reader.permissions) == 26
    error_message = "costguard.reader has ${length(stackit_authorization_organization_custom_role.reader.permissions)} permissions, expected 26."
  }
  assert {
    condition     = length(stackit_resourcemanager_project.costguard) == 1 && stackit_resourcemanager_project.costguard[0].parent_container_id == var.organization_id
    error_message = "The project must be created directly under the organization."
  }
  assert {
    condition     = stackit_server.costguard.agent.provisioning_policy == "NEVER" && stackit_server.costguard.boot_volume.performance_class == "storage_premium_perf0"
    error_message = "Server Agent must be off and the boot volume on perf0."
  }
  assert {
    condition     = length(stackit_public_ip.break_glass) == 0 && length(stackit_security_group_rule.break_glass_ssh) == 0 && stackit_server.costguard.keypair_name == null
    error_message = "Without break_glass: no public IP, no SSH rule, no key pair."
  }
}

run "user_data_contents" {
  command = apply

  variables {
    skip = { projects = ["prod: \"main\""] } # must not break the YAML
  }

  assert {
    condition     = startswith(module.cloud_init.user_data, "#cloud-config\n")
    error_message = "The user data must start with #cloud-config."
  }
  assert {
    condition     = yamldecode(one([for f in yamldecode(module.cloud_init.user_data).write_files : f.content if f.path == "/etc/costguard/config.yaml"])).skip.projects == ["prod: \"main\""]
    error_message = "config.yaml must survive special characters in names."
  }
  assert {
    condition = one([for f in yamldecode(module.cloud_init.user_data).write_files : f if f.path == "/etc/costguard/costguard.env"]) == {
      path        = "/etc/costguard/costguard.env"
      permissions = "0600"
      content     = "COSTGUARD_SERVICE_ACCOUNT_EMAIL='costguard-ab12cd@sa.stackit.cloud'\nCOSTGUARD_WEBHOOK_URL='https://hooks.slack.com/services/T0/B0/secret'\nLOG_LEVEL='info'\n"
    }
    error_message = "The environment file must be 0600 with the webhook, the service account email and the log level."
  }
  assert {
    condition = jsondecode(one([for f in yamldecode(module.cloud_init.user_data).write_files : f.content if f.path == "/etc/costguard/install.json"])) == {
      version = "v0.1.0-test"
      url     = "https://downloads.example/{version}/"
      sha256  = { amd64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" }
      output  = "slack"
    }
    error_message = "install.json must carry the version, URL, pinned hash and chat type."
  }
  assert {
    condition     = contains(yamldecode(module.cloud_init.user_data).runcmd, ["systemctl", "mask", "ssh.service", "ssh.socket"]) && contains(yamldecode(module.cloud_init.user_data).runcmd, ["systemctl", "mask", "--now", "sshd-unix-local.socket"]) && contains([for f in yamldecode(module.cloud_init.user_data).write_files : f.path], "/etc/ssh/sshd_not_to_be_run")
    error_message = "Without break_glass, sshd must never start and be masked."
  }
  assert {
    condition     = contains(yamldecode(module.cloud_init.user_data).runcmd, ["systemctl", "start", "--no-block", "costguard@boot.service"])
    error_message = "The boot run must be started (once, from runcmd)."
  }
  assert {
    condition     = !can(yamldecode(module.cloud_init.user_data).bootcmd)
    error_message = "No bootcmd: it can hang the boot."
  }
  assert {
    condition     = [for u in yamldecode(module.cloud_init.user_data).users : u.name] == ["costguard"]
    error_message = "Without break_glass only the costguard system user exists."
  }
}

# ---- delete on ----

run "delete_on" {
  command = apply

  variables {
    features = { delete = { enabled = true } }
  }

  assert {
    condition     = module.cloud_init.config.deleteEnabled && module.cloud_init.config.deleteRunAt == "Tuesday 08:00 (Europe/Berlin)"
    error_message = "deleteRunAt: ${try(module.cloud_init.config.deleteRunAt, "missing")}"
  }
  assert {
    condition     = [for t in module.cloud_init.timers : "${t.name}=${t.subcommand}"] == ["report=flag", "delete=delete"]
    error_message = "With delete on, the report timer runs flag, plus a delete timer."
  }
  assert {
    condition     = stackit_authorization_organization_role_assignment.cleaner[0].role == "costguard.cleaner" && length(stackit_authorization_organization_custom_role.cleaner[0].permissions) == 9
    error_message = "With delete on, costguard.cleaner (9 permissions) must be assigned."
  }
}

run "several_days" {
  command = apply

  variables {
    time_zone = "UTC"
    features = {
      report = { days = ["Fri", "Mon", "Wed"], time = "07:30" }
      delete = { enabled = true, days = ["Thu", "Tue"], time = "09:00" }
    }
  }

  assert {
    condition     = module.cloud_init.config.reportRunAt == "Monday, Wednesday and Friday 07:30 (UTC)" && module.cloud_init.config.deleteRunAt == "Tuesday and Thursday 09:00 (UTC)"
    error_message = "Texts: ${module.cloud_init.config.reportRunAt} / ${module.cloud_init.config.deleteRunAt}"
  }
  assert {
    condition     = module.cloud_init.timers[0].on_calendar == "Mon,Wed,Fri *-*-* 07:30:00 UTC" && module.cloud_init.timers[1].on_calendar == "Tue,Thu *-*-* 09:00:00 UTC"
    error_message = "Calendar specs: ${module.cloud_init.timers[0].on_calendar} / ${module.cloud_init.timers[1].on_calendar}"
  }
  assert {
    condition     = module.cloud_init.delete_gaps_hours == [25.5, 25.5]
    error_message = "Gaps: ${jsonencode(module.cloud_init.delete_gaps_hours)}"
  }
}

run "every_day" {
  command = plan

  variables {
    features = { report = { days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"] } }
  }

  assert {
    condition     = module.cloud_init.config.reportRunAt == "every day 08:00 (Europe/Berlin)"
    error_message = "reportRunAt: ${module.cloud_init.config.reportRunAt}"
  }
}

# ---- Rules across settings ----

run "delete_needs_report" {
  command = plan
  variables {
    features = { report = { enabled = false }, delete = { enabled = true } }
  }
  expect_failures = [terraform_data.settings]
}

run "delete_too_soon_after_report" {
  command = plan
  variables {
    features = { report = { time = "08:00" }, delete = { enabled = true, days = ["Mon"], time = "20:00" } }
  }
  expect_failures = [terraform_data.settings]
}

run "delete_at_the_same_time_as_report" {
  command = plan
  variables {
    features = { report = { days = ["Tue"] }, delete = { enabled = true, days = ["Tue"] } }
  }
  expect_failures = [terraform_data.settings]
}

run "delete_across_the_week_boundary" {
  command = plan
  variables {
    # Sunday 20:00 report, Monday 18:00 delete: 22 hours.
    features = { report = { days = ["Sun"], time = "20:00" }, delete = { enabled = true, days = ["Mon"], time = "18:00" } }
  }
  assert {
    condition     = module.cloud_init.delete_gaps_hours == [22]
    error_message = "Gap: ${jsonencode(module.cloud_init.delete_gaps_hours)}"
  }
}

# Every release tag, and main between releases, pins a release (make pin
# writes 025-release.tf); without binary_override the server installs it.
run "release_pinned" {
  command = plan
  variables {
    binary_override = null
    download_url    = null
  }
  assert {
    condition     = can(regex("^v[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$", local.release.version)) && keys(local.release.sha256) == ["amd64", "arm64"] && alltrue([for h in values(local.release.sha256) : can(regex("^[0-9a-f]{64}$", h))])
    error_message = "025-release.tf must pin a version and a SHA-256 per architecture: make pin VERSION=vX.Y.Z."
  }
  assert {
    condition     = local.release.download_url == "https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases/download/apps%2Fcostguard%2F{version}/"
    error_message = "The pin must point to the Forgejo release of tag apps/costguard/<version>, slashes escaped: ${local.release.download_url}"
  }
  assert {
    condition = jsondecode(one([for f in yamldecode(module.cloud_init.user_data).write_files : f.content if f.path == "/etc/costguard/install.json"])) == {
      version = local.release.version
      url     = local.release.download_url
      sha256  = local.release.sha256
      output  = "slack"
    }
    error_message = "Without binary_override, the server must install the pinned release."
  }
}

# ---- Typos and wrong values fail ----

run "typo_feature_name" {
  command = plan
  variables {
    features = { delet = { enabled = true } }
  }
  expect_failures = [var.features]
}

run "typo_feature_key" {
  command = plan
  variables {
    features = { delete = { enabled = true, dyas = ["Wed"] } }
  }
  expect_failures = [var.features]
}

run "quoted_bool" {
  command = plan
  variables {
    features = { report = { enabled = "false" } }
  }
  expect_failures = [var.features]
}

run "long_day_name" {
  command = plan
  variables {
    features = { report = { days = ["Monday"] } }
  }
  expect_failures = [var.features]
}

run "bad_time" {
  command = plan
  variables {
    features = { report = { time = "8:00" } }
  }
  expect_failures = [var.features]
}

run "typo_in_skip" {
  command = plan
  variables {
    skip = { projcts = ["prod"] }
  }
  expect_failures = [var.skip]
}

run "typo_in_scope" {
  command = plan
  variables {
    scope = { folder = ["team-a"] }
  }
  expect_failures = [var.scope]
}

run "webhook_with_quote" {
  command = plan
  variables {
    webhook_url = "https://hooks.slack.com/services/T0/B0/x'y"
  }
  expect_failures = [var.webhook_url]
}

run "webhook_http" {
  command = plan
  variables {
    webhook_url = "http://hooks.slack.com/services/T0/B0/x"
  }
  expect_failures = [var.webhook_url]
}

run "key_file_missing" {
  command = plan
  variables {
    service_account_key_path = "~/does-not-exist/deployer-key.json"
  }
  expect_failures = [var.service_account_key_path]
}

run "key_file_present" {
  command = plan
  variables {
    service_account_key_path = "010-provider.tf" # any existing file; the mocked provider ignores it
  }
}

run "project_owner_needed_for_new_project" {
  command = plan
  variables {
    project_owner_email = null
  }
  expect_failures = [var.project_owner_email]
}

run "binary_override_hash" {
  command = plan
  variables {
    binary_override = { version = "v0.1.0", sha256 = { amd64 = "abc" } }
  }
  expect_failures = [var.binary_override]
}

# ---- Warnings ----

run "webhook_of_another_chat" {
  command = plan
  variables {
    output = "teams"
  }
  expect_failures = [check.webhook_matches_output]
}

run "teams_workflow_webhook" {
  command = plan
  variables {
    output      = "teams"
    webhook_url = "https://prod-12.westeurope.logic.azure.com:443/workflows/abc/triggers/manual/paths/invoke?api-version=2016-06-01&sig=x"
  }
}

run "project_under_a_folder" {
  command = plan
  variables {
    parent_container_id = "22222222-0000-4000-8000-000000000000"
  }
  expect_failures = [check.project_parent]
}

run "run_during_reboot_window" {
  command = plan
  variables {
    features = { report = { time = "04:00" } }
  }
  expect_failures = [check.reboot_window]
}

# ---- Options ----

run "existing_project_and_network" {
  command = apply
  variables {
    project_id          = "33333333-0000-4000-8000-000000000000"
    network_id          = "44444444-0000-4000-8000-000000000000"
    project_owner_email = null
  }
  assert {
    condition     = length(stackit_resourcemanager_project.costguard) == 0 && length(stackit_network.costguard) == 0
    error_message = "With project_id and network_id nothing new is created for them."
  }
  assert {
    condition     = stackit_network_interface.costguard.network_id == var.network_id && stackit_server.costguard.project_id == var.project_id
    error_message = "The server must use the given project and network."
  }
}

run "break_glass" {
  command = apply
  variables {
    break_glass = { public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyForTestsOnly test", allowed_cidr = "203.0.113.7/32" }
  }
  assert {
    condition     = length(stackit_public_ip.break_glass) == 1 && stackit_security_group_rule.break_glass_ssh[0].ip_range == "203.0.113.7/32" && stackit_server.costguard.keypair_name == "costguard-break-glass"
    error_message = "break_glass needs the public IP, the SSH rule for the CIDR and the key pair."
  }
  assert {
    condition     = !contains(yamldecode(module.cloud_init.user_data).runcmd, ["systemctl", "mask", "ssh.service", "ssh.socket"]) && !contains([for f in yamldecode(module.cloud_init.user_data).write_files : f.path], "/etc/ssh/sshd_not_to_be_run")
    error_message = "With break_glass, sshd stays."
  }
  assert {
    condition     = yamldecode(module.cloud_init.user_data).users[0] == "default"
    error_message = "With break_glass the default user (debian) gets the key."
  }
}

run "break_glass_open_to_the_world" {
  command = plan
  variables {
    break_glass = { public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyForTestsOnly", allowed_cidr = "0.0.0.0/0" }
  }
  expect_failures = [var.break_glass]
}
