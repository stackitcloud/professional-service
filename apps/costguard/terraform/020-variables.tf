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

# Everything goes into terraform.tfvars, including the deployer's key path and
# the webhook URL, so nothing has to be exported. terraform.tfvars is
# git-ignored and must never be committed: it holds the webhook URL.
#
# Null checks use `x == null ? true : ...`, not `||`: Terraform before 1.12
# evaluates both sides of || and fails on an attribute of null.
#
# Terraform silently drops misspelled keys of object variables, so scope,
# skip and features are checked key by key: a typo such as
# skip = { projcts = [...] } must fail, not silently remove a protection.

# ---- Login ----

variable "service_account_key_path" {
  description = "The deployer service account's key file (JSON), e.g. ~/.stackit/deployer-key.json; keep it outside the repository. Default: STACKIT_SERVICE_ACCOUNT_KEY_PATH or ~/.stackit/credentials.json."
  type        = string
  default     = null

  validation {
    condition     = var.service_account_key_path == null ? true : fileexists(pathexpand(var.service_account_key_path))
    error_message = "service_account_key_path: the key file does not exist."
  }
}

# ---- Where costguard runs ----

variable "organization_id" {
  description = "The STACKIT organization costguard looks after (UUID). The custom roles are created and assigned on it."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", var.organization_id))
    error_message = "organization_id must be a lowercase UUID."
  }
}

variable "project_id" {
  description = "An existing project for costguard (e.g. a landing zone tooling project). Default: a new project \"costguard\" is created."
  type        = string
  default     = null

  validation {
    condition     = var.project_id == null ? true : can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", var.project_id))
    error_message = "project_id must be a lowercase UUID."
  }
}

variable "parent_container_id" {
  description = "Folder or organization the new project is created under. Default: the organization (recommended: whoever controls a parent folder controls costguard)."
  type        = string
  default     = null
}

variable "project_owner_email" {
  description = "Becomes owner of the new project: the deployer service account's email, not a person. Required unless project_id is set."
  type        = string
  default     = null

  validation {
    condition     = var.project_id != null || can(regex("^[^@\\s]+@[^@\\s]+$", var.project_owner_email))
    error_message = "project_owner_email is required when a new project is created: the deployer service account's email."
  }
}

variable "network_id" {
  description = "An existing network in the project (e.g. one in a STACKIT Network Area). Default: a new small network is created."
  type        = string
  default     = null

  validation {
    condition     = var.network_id == null ? true : can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", var.network_id))
    error_message = "network_id must be a lowercase UUID."
  }
}

variable "network_ipv4_prefix" {
  description = "CIDR of the new network (STACKIT allows /8 to /29)."
  type        = string
  default     = "10.64.0.0/28"

  validation {
    condition     = can(cidrhost(var.network_ipv4_prefix, 0)) && tonumber(split("/", var.network_ipv4_prefix)[1]) >= 8 && tonumber(split("/", var.network_ipv4_prefix)[1]) <= 29
    error_message = "network_ipv4_prefix must be an IPv4 CIDR between /8 and /29."
  }
}

variable "dns_nameservers" {
  description = "DNS servers of the new network."
  type        = list(string)
  default     = ["9.9.9.9"]
}

variable "region" {
  description = "STACKIT region of the server."
  type        = string
  default     = "eu01"
}

variable "availability_zone" {
  description = "Availability zone of the server (eu01-m, the metro zone, costs more)."
  type        = string
  default     = "eu01-1"
}

variable "machine_type" {
  description = "Server flavor. t2i.1 (1 vCPU, 1 GB) is enough and the cheapest current one."
  type        = string
  default     = "t2i.1"
}

variable "image_name" {
  description = "Exact name of the public image. STACKIT rebuilds it daily; a server keeps its image until it is replaced."
  type        = string
  default     = "Debian 13"
}

variable "boot_volume_gb" {
  description = "Boot volume size in GB."
  type        = number
  default     = 10
}

variable "boot_volume_performance_class" {
  description = "Boot volume performance class. Without one the API picks storage_premium_perf1 (about 5.55 EUR/month more)."
  type        = string
  default     = "storage_premium_perf0"
}

variable "time_zone" {
  description = "IANA time zone of the schedules, the messages and the server."
  type        = string
  default     = "Europe/Berlin"

  validation {
    condition     = can(regex("^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z0-9_+-]+)*$", var.time_zone)) && var.time_zone != "Local"
    error_message = "time_zone must be an IANA time zone such as Europe/Berlin."
  }
}

variable "break_glass" {
  description = "Temporary SSH access for debugging: a public key and the only CIDR allowed to connect. Adds a public IP, sshd and port 22. Toggling it replaces the server. Remove it and apply to close again."
  type = object({
    public_key   = string
    allowed_cidr = string
  })
  default = null

  validation {
    condition     = var.break_glass == null ? true : can(regex("^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp(256|384|521)) [A-Za-z0-9+/=]+( .*)?$", var.break_glass.public_key))
    error_message = "break_glass.public_key must be one OpenSSH public key line (ssh-ed25519 ..., ssh-rsa ... or ecdsa-sha2-...)."
  }

  validation {
    condition     = var.break_glass == null ? true : can(cidrhost(var.break_glass.allowed_cidr, 0)) && !startswith(var.break_glass.allowed_cidr, "0.0.0.0/")
    error_message = "break_glass.allowed_cidr must be an IPv4 CIDR and not 0.0.0.0/x: allow only your own address, e.g. 203.0.113.7/32."
  }
}

# ---- Chat ----

variable "output" {
  description = "Chat type of the webhook: teams, slack or googlechat."
  type        = string

  validation {
    condition     = contains(["teams", "slack", "googlechat"], var.output)
    error_message = "output must be teams, slack or googlechat."
  }
}

variable "webhook_url" {
  description = "The chat webhook. A secret: keep it in the git-ignored terraform.tfvars (or TF_VAR_webhook_url), never in a committed file."
  type        = string
  sensitive   = true

  # Written single-quoted into a systemd EnvironmentFile, so no quotes,
  # backslashes, spaces or line breaks.
  validation {
    condition     = can(regex("^https://[^\\s'\"\\\\]+$", var.webhook_url))
    error_message = "webhook_url must be an https URL without spaces, quotes or backslashes."
  }
}

# ---- What costguard does ----

variable "features" {
  description = <<-EOT
    The features, each with its own switch and schedule (days Mon..Sun, time HH:MM in time_zone):
      report:  posts the cleanup report; with delete on it also labels new candidates delete=true.
               Default { enabled = true, days = ["Mon"], time = "08:00" }.
      delete:  deletes what is labelled delete=true; needs report, and each delete run must come at
               least 20 hours after the report run before it. Default { enabled = false, days = ["Tue"], time = "08:00" }.
      budgets: monthly budgets; each run posts every budget whose month to date is at or above a threshold
               (percent of the limit), again in every run until the month ends. The 1st posts nothing (no
               cost of the month yet). STACKIT has a day's costs after 07:30 UTC the next day; earlier runs
               see them a day later.
               Default { enabled = false, days = ["Mon", "Tue", "Wed", "Thu", "Fri"], time = "10:00",
               thresholds = [80, 100], limits = [] }.
               Each limit: name, exactly one target (organization = true, folder = "<ID or name>" or
               project = "<ID or name>"), monthly_eur, and optionally its own thresholds.
    Keys left out keep their default.
  EOT
  # any, not object(): an object type would silently drop misspelled keys.
  type    = any
  default = {}

  validation {
    condition     = can(keys(var.features)) && alltrue([for f in try(keys(var.features), []) : contains(["report", "delete", "budgets"], f)])
    error_message = "features may only contain report, delete and budgets (check the spelling)."
  }

  validation {
    condition = alltrue([for f, v in try(tomap(var.features), var.features, {}) :
      can(keys(v)) && alltrue([for k in try(keys(v), []) : contains(f == "budgets" ? ["enabled", "days", "time", "thresholds", "limits"] : ["enabled", "days", "time"], k)])
    ])
    error_message = "report and delete take only the keys enabled, days, time; budgets enabled, days, time, thresholds, limits (check the spelling)."
  }

  validation {
    condition     = alltrue([for f, v in try(tomap(var.features), var.features, {}) : try(v.enabled == true || v.enabled == false, true)])
    error_message = "features.<name>.enabled must be true or false (without quotes)."
  }

  validation {
    condition = alltrue([for f, v in try(tomap(var.features), var.features, {}) :
      !can(v.days) || try(
        length(v.days) > 0 &&
        length(distinct(v.days)) == length(v.days) &&
        alltrue([for d in v.days : contains(["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"], d)]),
      false)
    ])
    error_message = "features.<name>.days must be a non-empty list of distinct days written Mon, Tue, Wed, Thu, Fri, Sat, Sun."
  }

  validation {
    condition     = alltrue([for f, v in try(tomap(var.features), var.features, {}) : !can(v.time) || can(regex("^([01][0-9]|2[0-3]):[0-5][0-9]$", v.time))])
    error_message = "features.<name>.time must be HH:MM (24 hours), e.g. \"08:00\"."
  }

  # ---- budgets ----

  validation {
    condition = alltrue([for t in concat(
      can(var.features.budgets.thresholds) ? [var.features.budgets.thresholds] : [],
      [for l in try(var.features.budgets.limits, []) : l.thresholds if can(l.thresholds)],
      ) : try(
      !can(keys(t)) && length(t) > 0 &&
      alltrue([for x in t : floor(x) == x && x >= 1 && x <= 1000]) &&
      alltrue([for i in range(1, length(t)) : t[i] > t[i - 1]]),
    false)])
    error_message = "Budget thresholds must be a non-empty list of whole percentages from 1 to 1000, ascending without repeats, e.g. [80, 100]."
  }

  validation {
    condition = !can(var.features.budgets.limits) || try(
      !can(keys(var.features.budgets.limits)) &&
      alltrue([for l in var.features.budgets.limits :
        can(keys(l)) && alltrue([for k in keys(l) : contains(["name", "organization", "folder", "project", "monthly_eur", "thresholds"], k)])
      ]),
    false)
    error_message = "features.budgets.limits must be a list of objects with only these keys: name, organization, folder, project, monthly_eur, thresholds (check the spelling)."
  }

  validation {
    condition = try(alltrue([for l in try(var.features.budgets.limits, []) : length([for t in [
      try(l.organization == true, false),
      try(trimspace(l.folder) != "", false),
      try(trimspace(l.project) != "", false),
    ] : t if t]) == 1 && try(l.organization == true, true)]), false)
    error_message = "Each budget needs exactly one target: organization = true, folder = \"<ID or name>\" or project = \"<ID or name>\"."
  }

  validation {
    condition = try(
      alltrue([for l in try(var.features.budgets.limits, []) : trimspace(l.name) != "" && l.monthly_eur > 0]) &&
      length(distinct([for l in try(var.features.budgets.limits, []) : lower(trimspace(l.name))])) == length(try(var.features.budgets.limits, [])),
    false)
    error_message = "Each budget needs a name (unique) and monthly_eur above 0."
  }

  validation {
    condition     = try(var.features.budgets.enabled, false) != true || try(length(var.features.budgets.limits), 0) > 0
    error_message = "features.budgets is enabled but has no limits: add at least one budget."
  }
}

variable "scope" {
  description = "Folders and projects (IDs or exact names) costguard looks at. Empty: the whole organization. Every entry must match exactly one container."
  type        = map(list(string))
  default     = {}

  validation {
    condition     = alltrue([for k in keys(var.scope) : contains(["folders", "projects"], k)])
    error_message = "scope may only contain the keys folders and projects (check the spelling)."
  }
}

variable "skip" {
  description = "Folders and projects (IDs or exact names) costguard never looks at. A name matching several containers skips all of them; an entry matching nothing blocks every write."
  type        = map(list(string))
  default     = {}

  validation {
    condition     = alltrue([for k in keys(var.skip) : contains(["folders", "projects"], k)])
    error_message = "skip may only contain the keys folders and projects (check the spelling)."
  }
}

variable "regions" {
  description = "Regions costguard scans."
  type        = list(string)
  default     = ["eu01"]

  validation {
    condition     = length(var.regions) > 0 && alltrue([for r in var.regions : can(regex("^[a-z]{2}[0-9]{2}$", r))])
    error_message = "regions must list STACKIT region IDs such as eu01."
  }
}

variable "warn_empty_after_days" {
  description = "Minimum age in days of an empty project or network area before the report warns about it; 0 switches these warnings off."
  type        = number
  default     = 30

  validation {
    condition     = var.warn_empty_after_days == 0 || var.warn_empty_after_days >= 7
    error_message = "warn_empty_after_days must be 0 (off) or at least 7."
  }
}

variable "prices" {
  description = "Prices for the savings estimate (net EUR, STACKIT price list v1.0.43)."
  type = object({
    public_ip_monthly_eur = optional(number, 2.92)
    volume_gb_monthly_eur = optional(number, 0.065)
  })
  default = {}

  validation {
    condition     = var.prices.public_ip_monthly_eur >= 0 && var.prices.volume_gb_monthly_eur >= 0
    error_message = "prices must not be negative."
  }
}

variable "log_level" {
  description = "costguard's log level in the journal: debug, info, warn or error."
  type        = string
  default     = "info"

  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "log_level must be debug, info, warn or error."
  }
}

# ---- The binary (normally pinned in 000-release.tf) ----

variable "binary_override" {
  description = "Only for test builds, or to pin hashes you checked yourself: runs another binary than the release this code pins, with these hashes (nothing is read from a SHA256SUMS then). version as in the file names, sha256 per architecture (amd64, arm64)."
  type = object({
    version = string
    sha256  = map(string)
  })
  default = null

  validation {
    condition = var.binary_override == null ? true : (
      can(regex("^[A-Za-z0-9][A-Za-z0-9._-]*$", var.binary_override.version)) &&
      length(var.binary_override.sha256) > 0 &&
      alltrue([for k, v in var.binary_override.sha256 : contains(["amd64", "arm64"], k) && can(regex("^[0-9a-f]{64}$", v))])
    )
    error_message = "binary_override needs a version (letters, digits, . _ -) and sha256 = { amd64 = \"<64 hex>\" } (keys amd64/arm64)."
  }
}

variable "download_url" {
  description = "Where the server downloads the binary: an https address; {version} is replaced, the file name costguard_<version>_linux_<arch> is appended. Without binary_override, Terraform reads SHA256SUMS from the same place when it plans. Default: the release location in 000-release.tf."
  type        = string
  default     = null

  validation {
    condition     = var.download_url == null ? true : can(regex("^https://[^\\s'\"\\\\]+$", var.download_url))
    error_message = "download_url must be an https URL without spaces or quotes."
  }
}
