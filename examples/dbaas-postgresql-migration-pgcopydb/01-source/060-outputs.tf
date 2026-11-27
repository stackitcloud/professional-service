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

# 02-postgres-flex and 03-migration read these through terraform_remote_state.

output "project_id" {
  value = stackit_resourcemanager_project.source.project_id
}

output "security_group_id" {
  value = stackit_security_group.firewall.security_group_id
}

output "public_ip" {
  value = stackit_public_ip.source.ip
}

output "ssh_command" {
  value = "ssh ubuntu@${stackit_public_ip.source.ip}"
}

output "source_host" {
  value = "127.0.0.1"
}

output "source_port" {
  value = 5432
}

output "source_user" {
  description = "Superuser DBMS"
  value       = "postgres"
}

output "source_password" {
  description = "Password of the source superuser"
  value       = random_password.postgres.result
  sensitive   = true
}

output "source_databases" {
  value = ["appdb", "analyticsdb"]
}
