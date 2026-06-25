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
