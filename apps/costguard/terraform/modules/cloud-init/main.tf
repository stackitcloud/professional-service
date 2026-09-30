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

# Renders the server's cloud-init user data. A pure function (no providers),

# The whole cloud-config is built as an object and yamlencode()d: no value
# (a skip entry, the webhook URL) can break the YAML. Nothing here may print
# a secret: cloud-init's output goes to the serial console, which the IaaS
# API shows to anyone who may read the server.

locals {
  # ---- Features ----

  week      = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
  day_names = { Mon = "Monday", Tue = "Tuesday", Wed = "Wednesday", Thu = "Thursday", Fri = "Friday", Sat = "Saturday", Sun = "Sunday" }

  feature_defaults = {
    report = { enabled = true, days = ["Mon"], time = "08:00" }
    delete = { enabled = false, days = ["Tue"], time = "08:00" }
    budgets = { enabled = false, days = ["Mon", "Tue", "Wed", "Thu", "Fri"], time = "10:00", thresholds = [80, 100], limits = [] }
  }
  features = { for name, defaults in local.feature_defaults : name => merge(defaults, try(var.settings.features[name], {})) }

  report_enabled  = local.features.report.enabled
  delete_enabled  = local.features.delete.enabled
  budgets_enabled = local.features.budgets.enabled

  feature_days = { for name, f in local.features : name => [for d in local.week : d if contains(f.days, d)] }
  schedules = { for name, f in local.features : name => {
    minutes = [for d in local.feature_days[name] :
      index(local.week, d) * 1440 + tonumber(substr(f.time, 0, 2)) * 60 + tonumber(substr(f.time, 3, 2))
    ]
    on_calendar = "${length(local.feature_days[name]) == 7 ? "" : "${join(",", local.feature_days[name])} "}*-*-* ${f.time}:00 ${var.settings.time_zone}"
    text = format("%s %s (%s)",
      length(local.feature_days[name]) == 7 ? "every day" : join(",", local.feature_days[name]) == "Mon,Tue,Wed,Thu,Fri" ? "Monday to Friday" : join(" and ", compact([
        join(", ", [for d in slice(local.feature_days[name], 0, length(local.feature_days[name]) - 1) : local.day_names[d]]),
        local.day_names[local.feature_days[name][length(local.feature_days[name]) - 1]],
      ])),
      f.time, var.settings.time_zone,
    )
  } }

  delete_gaps_hours = [for d in local.schedules.delete.minutes :
    min([for r in local.schedules.report.minutes : (d - r + 10080) % 10080]...) / 60
  ]

  timers = concat(
    local.report_enabled ? [{
      name        = "report"
      subcommand  = local.delete_enabled ? "flag" : "report"
      on_calendar = local.schedules.report.on_calendar
      description = "costguard report run, ${local.schedules.report.text}"
    }] : [],
    local.delete_enabled ? [{
      name        = "delete"
      subcommand  = "delete"
      on_calendar = local.schedules.delete.on_calendar
      description = "costguard delete run, ${local.schedules.delete.text}"
    }] : [],
    local.budgets_enabled ? [{
      name        = "budgets"
      subcommand  = "budgets"
      on_calendar = local.schedules.budgets.on_calendar
      description = "costguard budgets run, ${local.schedules.budgets.text}"
    }] : [],
  )

  # ---- costguard's config (the keys of internal/config) ----

  config = merge(
    {
      organizationId = var.settings.organization_id
      scope = {
        folders  = lookup(var.settings.scope, "folders", [])
        projects = lookup(var.settings.scope, "projects", [])
      }
      skip = {
        folders  = lookup(var.settings.skip, "folders", [])
        projects = lookup(var.settings.skip, "projects", [])
      }
      regions            = var.settings.regions
      output             = var.settings.output
      warnEmptyAfterDays = var.settings.warn_empty_after_days
      reportEnabled      = local.report_enabled
      deleteEnabled      = local.delete_enabled
      timeZone           = var.settings.time_zone
      prices = {
        publicIpMonthlyEur = var.settings.prices.public_ip_monthly_eur
        volumeGbMonthlyEur = var.settings.prices.volume_gb_monthly_eur
      }
    },
    local.delete_enabled ? { deleteRunAt = local.schedules.delete.text } : {},
    local.report_enabled ? { reportRunAt = local.schedules.report.text } : {},
    local.budgets_enabled ? {
      budgets = {
        days = local.feature_days.budgets
        time = local.features.budgets.time
        limits = [for l in local.features.budgets.limits : merge(
          {
            name       = trimspace(l.name)
            monthlyEur = tonumber(l.monthly_eur)
            thresholds = try(l.thresholds, local.features.budgets.thresholds)
          },
          try(l.organization, false) ? { organization = true } : {},
          can(l.folder) ? { folder = trimspace(tostring(l.folder)) } : {},
          can(l.project) ? { project = trimspace(tostring(l.project)) } : {},
        )]
      }
    } : {},
  )

  environment = {
    COSTGUARD_WEBHOOK_URL           = var.webhook_url
    COSTGUARD_SERVICE_ACCOUNT_EMAIL = var.service_account_email
    LOG_LEVEL                       = var.settings.log_level
  }

  timer_units = { for t in local.timers : "costguard-${t.name}.timer" => templatefile("${path.module}/files/timer.tftpl", {
    description = t.description
    on_calendar = t.on_calendar
    unit        = "costguard@${t.subcommand}.service"
  }) }

  environment_file = join("", [for k in sort(keys(local.environment)) : "${k}='${local.environment[k]}'\n"])

  files = concat(
    [
      { path = "/etc/costguard/config.yaml", permissions = "0644", content = yamlencode(local.config) },
      { path = "/etc/costguard/costguard.env", permissions = "0600", content = local.environment_file },
      {
        path        = "/etc/costguard/install.json"
        permissions = "0644"
        content     = jsonencode({ version = var.binary.version, url = var.binary.url, sha256 = var.binary.sha256, output = var.settings.output })
      },
      { path = "/usr/local/lib/costguard/costguard_install.py", permissions = "0755", content = file("${path.module}/files/costguard_install.py") },
      { path = "/etc/systemd/system/costguard-install.service", permissions = "0644", content = file("${path.module}/files/costguard-install.service") },
      { path = "/etc/systemd/system/costguard@.service", permissions = "0644", content = file("${path.module}/files/costguard@.service") },
      { path = "/etc/systemd/resolved.conf.d/50-costguard.conf", permissions = "0644", content = "[Resolve]\nLLMNR=no\nMulticastDNS=no\n" },
      {
        path        = "/etc/apt/apt.conf.d/52costguard-unattended-upgrades"
        permissions = "0644"
        content     = "Unattended-Upgrade::Automatic-Reboot \"true\";\nUnattended-Upgrade::Automatic-Reboot-WithUsers \"true\";\nUnattended-Upgrade::Automatic-Reboot-Time \"04:00\";\n"
      },
    ],
    [for name, content in local.timer_units : { path = "/etc/systemd/system/${name}", permissions = "0644", content = content }],
    [{
      path        = "/etc/ssh/sshd_not_to_be_run"
      permissions = "0644"
      content     = "costguard: no SSH\n"
    }],
  )

  ssh_commands = [
    ["systemctl", "disable", "--now", "ssh.service", "ssh.socket"],
    ["systemctl", "mask", "ssh.service", "ssh.socket"],
    ["systemctl", "mask", "--now", "sshd-unix-local.socket"],
  ]

  cloud_config = {
    timezone        = var.settings.time_zone
    package_update  = false
    package_upgrade = false
    disable_root    = true
    users = [{
      name           = "costguard"
      system         = true
      shell          = "/usr/sbin/nologin"
      lock_passwd    = true
      no_create_home = true
    }]
    write_files = local.files
    runcmd = concat(
      [
        ["systemctl", "mask", "stackit-server-agent.service"],
      ],
      local.ssh_commands,
      [
        ["systemctl", "restart", "systemd-resolved.service"],
        ["systemctl", "daemon-reload"],
      ],
      length(local.timer_units) > 0 ? [concat(["systemctl", "enable", "--now"], keys(local.timer_units))] : [],
      [
        ["systemctl", "start", "--no-block", "costguard@boot.service"],
      ],
    )
    final_message = "costguard: cloud-init finished after $UPTIME seconds"
  }
}
