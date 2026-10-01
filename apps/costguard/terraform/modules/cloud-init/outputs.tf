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

output "user_data" {
  description = "The #cloud-config user data. Sensitive: it holds the webhook URL."
  value       = "#cloud-config\n${yamlencode(local.cloud_config)}"
  sensitive   = true
}

output "features" {
  description = "The features with their defaults filled in."
  value       = local.features
}

output "schedule" {
  description = "When each enabled job runs, as the messages say it."
  value       = { for t in local.timers : t.name => local.schedules[t.name].text }
}

output "run_times" {
  description = "HH:MM of each enabled job."
  value       = { for t in local.timers : t.name => local.features[t.name].time }
}

output "delete_gaps_hours" {
  description = "For each delete run: hours since the report run before it."
  value       = local.delete_gaps_hours
}

output "config" {
  description = "The content of /etc/costguard/config.yaml."
  value       = local.config
}

output "timers" {
  description = "The systemd timers: name, subcommand, calendar spec."
  value       = local.timers
}
