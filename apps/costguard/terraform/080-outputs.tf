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


output "project_id" {
  description = "The project costguard runs in."
  value       = local.project_id
}

output "server_id" {
  description = "The server. Serial console: stackit server log <server_id> --project-id <project_id>"
  value       = stackit_server.costguard.server_id
}

output "service_account_email" {
  description = "costguard's identity; its roles are costguard.reader (and costguard.cleaner with delete on) on the organization."
  value       = stackit_service_account.costguard.email
}

output "schedule" {
  description = "When the enabled features run."
  value       = module.cloud_init.schedule
}

output "binary" {
  description = "The costguard version the server runs, where it downloads it from and the SHA-256 it accepts (compare with your own make dist build if you like: it is reproducible)."
  value       = { version = local.binary.version, download_url = local.binary.url, sha256 = local.binary.sha256 }
}

output "image" {
  description = "The OS image a new server would get (the running one keeps its image until it is replaced)."
  value       = { id = local.image_id, name = data.stackit_image_v2.os.name }
}
