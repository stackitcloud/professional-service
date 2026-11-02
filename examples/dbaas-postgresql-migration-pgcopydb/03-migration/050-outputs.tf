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

output "ssh_command" {
  description = "Log in to the migration host"
  value       = "ssh ubuntu@${local.source.public_ip}"
}

output "firewall_opened_to" {
  description = "Addresses of PostgreSQL Flex the firewall lets the source VM reach"
  value       = [for r in stackit_security_group_rule.egress_postgres_flex : r.ip_range]
}

output "databases" {
  description = "Databases to migrate, one run per database"
  value       = local.target.databases
}

output "next_steps" {
  description = "Runbook on the migration host, per database"
  value       = <<-EOT
    ssh ubuntu@${local.source.public_ip}
    cd /opt/migration/scripts
    sudo ./10-preflight.sh       <db>
    sudo ./20-prepare-target.sh  <db>
    sudo ./30-freeze-source.sh   <db> on
    sudo ./40-clone.sh           <db>      # ./45-progress.sh <db> in a second terminal
    sudo ./50-post.sh            <db>
    sudo ./60-verify.sh          <db>
    sudo ./60-verify.sh          <db> --compare-data   # optional, content of every table
    Databases: ${join(", ", local.target.databases)}
  EOT
}
