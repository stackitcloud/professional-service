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
  # Everything the runner needs, in one file
  migration_env = templatefile("${path.module}/templates/migration.env.tftpl", {
    source_host              = local.source.source_host
    source_port              = local.source.source_port
    source_user              = local.source.source_user
    source_password          = local.source.source_password
    target_host              = local.target.host
    target_port              = local.target.port
    target_user              = local.target.owner_user
    target_password          = local.target.owner_password
    app_users                = join(",", local.target.app_users)
    databases                = join(",", local.target.databases)
    target_storage_gb        = local.target.storage_size_gb
    target_backup_schedule   = local.target.backup_schedule
    table_jobs               = var.table_jobs
    index_jobs               = var.index_jobs
    split_tables_larger_than = var.split_tables_larger_than
  })

  runner_files = fileset("${path.module}/runner", "**")
  runner_hash  = sha1(join(",", [for f in sort(local.runner_files) : filesha1("${path.module}/runner/${f}")]))
}

# Copies the runner to the source VM and builds its image there. It does not
# start the migration (see README)
resource "terraform_data" "runner" {
  triggers_replace = {
    host      = local.source.public_ip
    runner    = local.runner_hash
    env       = sha1(local.migration_env)
    target_pg = local.target.postgres_version
  }

  connection {
    type        = "ssh"
    host        = local.source.public_ip
    user        = "ubuntu"
    private_key = var.ssh_private_key_path == null ? null : file(pathexpand(var.ssh_private_key_path))
    agent       = var.ssh_private_key_path == null
    timeout     = "10m"
  }

  # double check in case of fast deployments and 01-source was not yet finalized
  provisioner "remote-exec" {
    inline = [
      "cloud-init status --wait >/dev/null || true",
      "rm -rf /tmp/migration && mkdir -p /tmp/migration",
    ]
  }

  provisioner "file" {
    source      = "${path.module}/runner/"
    destination = "/tmp/migration"
  }

  provisioner "file" {
    content     = local.migration_env
    destination = "/tmp/migration/migration.env"
  }

  provisioner "remote-exec" {
    inline = [
      "set -e",
      "sudo install -d -m 0755 -o root -g root /opt/migration",
      "sudo cp -r /tmp/migration/. /opt/migration/",
      "rm -rf /tmp/migration",
      "sudo chown -R root:root /opt/migration",
      "sudo chmod 0600 /opt/migration/migration.env",
      "sudo chmod 0755 /opt/migration/scripts/*.sh",
      "sudo PG_MAJOR=${local.target.postgres_version} /opt/migration/scripts/00-build-runner.sh",
    ]
  }

  depends_on = [stackit_security_group_rule.egress_postgres_flex]
}
