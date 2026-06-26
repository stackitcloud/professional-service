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
  pg_username = stackit_postgresflex_user.db_owner.username
  pg_password = stackit_postgresflex_user.db_owner.password
  pg_host     = stackit_postgresflex_user.db_owner.host
  pg_port     = stackit_postgresflex_user.db_owner.port
  pg_database = stackit_postgresflex_database.this.name
}

output "pg_connection_uri" {
  description = "PostgreSQL Flex User Connection String"
  value       = "postgres://${local.pg_username}:${local.pg_password}@${local.pg_host}:${local.pg_port}/${local.pg_database}?sslmode=require"
  sensitive   = true
}
