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


# Offline tests of the root config: terraform test (no STACKIT access, no
# download; the providers are mocked). Covers the variable checks, the rules
# across settings, what the server gets (config, units, environment) and
# which resources each option creates.

# The release's SHA256SUMS. Mocks must be static (Terraform 1.9, OpenTofu),
# so the names carry a made-up version: the hashes are taken per
# architecture.
mock_provider "http" {
  mock_data "http" {
    defaults = {
      status_code   = 200
      response_body = "1111111111111111111111111111111111111111111111111111111111111111  costguard_v9.9.9_linux_amd64\n2222222222222222222222222222222222222222222222222222222222222222  costguard_v9.9.9_linux_arm64\n"
    }
  }
}

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

run "report_on_workdays" {
  command = plan

  variables {
    features = { report = { days = ["Fri", "Thu", "Wed", "Tue", "Mon"] } }
  }

  assert {
    condition     = module.cloud_init.config.reportRunAt == "Monday to Friday 08:00 (Europe/Berlin)"
    error_message = "reportRunAt: ${module.cloud_init.config.reportRunAt}"
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

# ---- budgets ----

run "budgets_on" {
  command = apply

  variables {
    features = {
      budgets = {
        enabled = true
        limits = [
          { name = "Whole organization", organization = true, monthly_eur = 20000 },
          { name = " Team A ", folder = "team-a", monthly_eur = 2500.5, thresholds = [50, 100] },
          { name = "Sandbox", project = "99999999-0000-4000-8000-0000000000aa", monthly_eur = 50 },
        ]
      }
    }
  }

  assert {
    condition = jsonencode(yamldecode(one([for f in yamldecode(module.cloud_init.user_data).write_files : f.content if f.path == "/etc/costguard/config.yaml"])).budgets) == jsonencode({
      days = ["Mon", "Tue", "Wed", "Thu", "Fri"]
      time = "10:00"
      limits = [
        { name = "Whole organization", organization = true, monthlyEur = 20000, thresholds = [80, 100] },
        { name = "Team A", folder = "team-a", monthlyEur = 2500.5, thresholds = [50, 100] },
        { name = "Sandbox", project = "99999999-0000-4000-8000-0000000000aa", monthlyEur = 50, thresholds = [80, 100] },
      ]
    })
    error_message = "config.yaml budgets: ${jsonencode(module.cloud_init.config.budgets)}"
  }
  assert {
    condition     = [for t in module.cloud_init.timers : "${t.name}=${t.subcommand}@${t.on_calendar}"] == ["report=report@Mon *-*-* 08:00:00 Europe/Berlin", "budgets=budgets@Mon,Tue,Wed,Thu,Fri *-*-* 10:00:00 Europe/Berlin"]
    error_message = "Timers: ${jsonencode(module.cloud_init.timers)}"
  }
  assert {
    condition     = module.cloud_init.config.reportEnabled && module.cloud_init.schedule.budgets == "Monday to Friday 10:00 (Europe/Berlin)"
    error_message = "reportEnabled must be true and budgets run Monday to Friday by default: ${module.cloud_init.schedule.budgets}"
  }
  assert {
    condition     = length(stackit_authorization_organization_custom_role.reader.permissions) == 26 && contains(stackit_authorization_organization_custom_role.reader.permissions, "cost-management.billing.get") && length(stackit_authorization_organization_custom_role.cleaner) == 0
    error_message = "Budgets need no new rights: costguard.reader already reads costs."
  }
}

run "budgets_default_thresholds_of_the_feature" {
  command = plan

  variables {
    time_zone = "UTC"
    features = {
      budgets = { enabled = true, time = "08:00", thresholds = [90], limits = [{ name = "Org", organization = true, monthly_eur = 1 }] }
    }
  }

  assert {
    condition     = module.cloud_init.config.budgets.limits[0].thresholds == [90] && module.cloud_init.config.budgets.time == "08:00"
    error_message = "A limit without thresholds takes the feature's: ${jsonencode(module.cloud_init.config.budgets)}"
  }
}

run "budget_quoted_amount" {
  command = plan

  variables {
    features = { budgets = { enabled = true, limits = [{ name = "Org", organization = true, monthly_eur = "100" }] } }
  }

  assert {
    condition     = jsonencode(module.cloud_init.config.budgets.limits[0]) == jsonencode({ monthlyEur = 100, name = "Org", organization = true, thresholds = [80, 100] })
    error_message = "A quoted amount must reach config.yaml as a number: ${jsonencode(module.cloud_init.config.budgets.limits[0])}"
  }
}

run "budget_quoted_threshold" {
  command = plan
  variables {
    features = { budgets = { enabled = true, thresholds = ["80"], limits = [{ name = "Org", organization = true, monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budgets_only" {
  command = plan

  variables {
    features = {
      report  = { enabled = false }
      budgets = { enabled = true, limits = [{ name = "Org", organization = true, monthly_eur = 100 }] }
    }
  }

  assert {
    condition     = [for t in module.cloud_init.timers : t.name] == ["budgets"] && module.cloud_init.config.reportEnabled == false && !contains(keys(module.cloud_init.config), "reportRunAt")
    error_message = "Budgets only: one budgets timer, reportEnabled false, no reportRunAt."
  }
}

run "budgets_off_writes_no_budgets" {
  command = plan

  variables {
    features = { budgets = { enabled = false, limits = [{ name = "Org", organization = true, monthly_eur = 100 }] } }
  }

  assert {
    condition     = !contains(keys(module.cloud_init.config), "budgets") && length([for t in module.cloud_init.timers : t if t.name == "budgets"]) == 0
    error_message = "With budgets off, the config has no budgets and there is no budgets timer."
  }
}

run "nothing_enabled" {
  command = plan
  variables {
    features = { report = { enabled = false } }
  }
  expect_failures = [terraform_data.settings]
}

run "budgets_during_reboot_window" {
  command = plan
  variables {
    features = { budgets = { enabled = true, time = "04:15", limits = [{ name = "Org", organization = true, monthly_eur = 1 }] } }
  }
  expect_failures = [check.reboot_window]
}

run "budgets_on_chosen_days" {
  command = plan

  variables {
    time_zone = "UTC"
    features  = { budgets = { enabled = true, days = ["Sun", "Wed"], time = "09:00", limits = [{ name = "Org", organization = true, monthly_eur = 1 }] } }
  }

  assert {
    condition     = module.cloud_init.config.budgets.days == ["Wed", "Sun"] && module.cloud_init.schedule.budgets == "Wednesday and Sunday 09:00 (UTC)"
    error_message = "Days in week order: ${jsonencode(module.cloud_init.config.budgets.days)}, ${module.cloud_init.schedule.budgets}"
  }
  assert {
    condition     = one([for t in module.cloud_init.timers : t.on_calendar if t.name == "budgets"]) == "Wed,Sun *-*-* 09:00:00 UTC"
    error_message = "Timers: ${jsonencode(module.cloud_init.timers)}"
  }
}

run "budgets_every_day" {
  command = plan

  variables {
    features = { budgets = { enabled = true, days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"], limits = [{ name = "Org", organization = true, monthly_eur = 1 }] } }
  }

  assert {
    condition     = module.cloud_init.schedule.budgets == "every day 10:00 (Europe/Berlin)" && one([for t in module.cloud_init.timers : t.on_calendar if t.name == "budgets"]) == "*-*-* 10:00:00 Europe/Berlin"
    error_message = "Every day: ${module.cloud_init.schedule.budgets}"
  }
}

run "budgets_bad_day" {
  command = plan
  variables {
    features = { budgets = { enabled = true, days = ["Monday"], limits = [{ name = "Org", organization = true, monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budgets_typo_key" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limit = [{ name = "Org", organization = true, monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budgets_without_limits" {
  command = plan
  variables {
    features = { budgets = { enabled = true } }
  }
  expect_failures = [var.features]
}

run "budget_typo_key" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "Org", organization = true, monthly_eur = 1, treshold = [50] }] } }
  }
  expect_failures = [var.features]
}

run "budget_two_targets" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "X", organization = true, folder = "team-a", monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budget_no_target" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "X", organization = false, monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budget_quoted_organization" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "X", organization = "true", monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budget_empty_folder" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "X", folder = " ", monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budget_zero_amount" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "X", organization = true, monthly_eur = 0 }] } }
  }
  expect_failures = [var.features]
}

run "budget_without_name" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ organization = true, monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budget_names_twice" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [
      { name = "Team", folder = "a", monthly_eur = 1 },
      { name = "team ", folder = "b", monthly_eur = 1 },
    ] } }
  }
  expect_failures = [var.features]
}

run "budget_thresholds_descending" {
  command = plan
  variables {
    features = { budgets = { enabled = true, thresholds = [100, 80], limits = [{ name = "X", organization = true, monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
}

run "budget_threshold_fraction" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "X", organization = true, monthly_eur = 1, thresholds = [80.5] }] } }
  }
  expect_failures = [var.features]
}

run "budget_threshold_out_of_range" {
  command = plan
  variables {
    features = { budgets = { enabled = true, limits = [{ name = "X", organization = true, monthly_eur = 1, thresholds = [0, 1001] }] } }
  }
  expect_failures = [var.features]
}

run "budget_thresholds_empty" {
  command = plan
  variables {
    features = { budgets = { enabled = true, thresholds = [], limits = [{ name = "X", organization = true, monthly_eur = 1 }] } }
  }
  expect_failures = [var.features]
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

# Every release tag, and main between releases, pins a released version
# (000-release.tf); without binary_override the server installs it, with
# the hashes from the release's SHA256SUMS (mocked: any version's names).
run "release_pinned" {
  command = apply
  variables {
    binary_override = null
    download_url    = null
  }
  assert {
    condition     = can(regex("^v[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$", local.release.version))
    error_message = "000-release.tf must pin a version vX.Y.Z: ${local.release.version}"
  }
  assert {
    condition     = local.release.download_url == "https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases/download/apps%2Fcostguard%2F{version}/"
    error_message = "The pin must point to the Forgejo release of tag apps/costguard/<version>, slashes escaped: ${local.release.download_url}"
  }
  assert {
    condition     = data.http.release_sums[0].url == "https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases/download/apps%2Fcostguard%2F${local.release.version}/SHA256SUMS"
    error_message = "SHA256SUMS URL: ${data.http.release_sums[0].url}"
  }
  assert {
    condition = jsondecode(one([for f in yamldecode(module.cloud_init.user_data).write_files : f.content if f.path == "/etc/costguard/install.json"])) == {
      version = local.release.version
      url     = local.release.download_url
      sha256 = {
        amd64 = "1111111111111111111111111111111111111111111111111111111111111111"
        arm64 = "2222222222222222222222222222222222222222222222222222222222222222"
      }
      output = "slack"
    }
    error_message = "Without binary_override, the server must install the pinned release with the hashes of its SHA256SUMS."
  }
  assert {
    condition     = output.binary.sha256.amd64 == "1111111111111111111111111111111111111111111111111111111111111111"
    error_message = "The binary output shows the hashes."
  }
}

run "own_download_location" {
  command = plan
  variables {
    binary_override = null
    download_url    = "https://files.example/costguard/{version}"
  }
  assert {
    condition     = data.http.release_sums[0].url == "https://files.example/costguard/${local.release.version}/SHA256SUMS"
    error_message = "SHA256SUMS comes from the same place as the binaries: ${data.http.release_sums[0].url}"
  }
}

run "release_not_published" {
  command = plan
  variables {
    binary_override = null
    download_url    = null
  }
  override_data {
    target = data.http.release_sums
    values = { status_code = 404, response_body = "Not Found" }
  }
  expect_failures = [data.http.release_sums[0]]
}

run "release_sums_incomplete" {
  command = plan
  variables {
    binary_override = null
    download_url    = null
  }
  override_data {
    target = data.http.release_sums
    values = {
      status_code   = 200
      response_body = "1111111111111111111111111111111111111111111111111111111111111111  costguard_v9.9.9_linux_amd64\n1111111111111111111111111111111111111111111111111111111111111111  costguard_v9.9.9_linux_amd64\n"
    }
  }
  expect_failures = [terraform_data.settings]
}

run "test_build_reads_no_release" {
  command = plan
  assert {
    condition     = length(data.http.release_sums) == 0 && jsonencode(local.binary.sha256) == jsonencode(var.binary_override.sha256)
    error_message = "With binary_override nothing is downloaded at plan time."
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
