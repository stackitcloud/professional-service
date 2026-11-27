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

# Superuser of the source DBMS
resource "random_password" "postgres" {
  length  = 32
  special = false
}

resource "stackit_key_pair" "admin" {
  name       = "${var.server_name}-admin"
  public_key = chomp(file(pathexpand(var.ssh_public_key_path)))
  labels     = var.labels
}

data "stackit_image_v2" "ubuntu" {
  project_id = stackit_resourcemanager_project.source.project_id
  name       = var.image_name
}

resource "stackit_network_interface" "source" {
  project_id         = stackit_resourcemanager_project.source.project_id
  network_id         = stackit_network.source.network_id
  name               = "${var.server_name}-nic"
  security_group_ids = [stackit_security_group.firewall.security_group_id]
  labels             = var.labels
}

resource "stackit_public_ip" "source" {
  project_id           = stackit_resourcemanager_project.source.project_id
  network_interface_id = stackit_network_interface.source.network_interface_id
  labels               = var.labels
}

locals {
  # The project is created in the same run, so the data source is only read at
  # apply time.
  image_id = element(split(",", data.stackit_image_v2.ubuntu.id), 2)

  # gz+b64 keeps the user data well below the size limit
  user_data = templatefile("${path.module}/cloud-init/source.yaml.tftpl", {
    postgres_image    = var.postgres_image
    postgres_password = random_password.postgres.result
    init_sh           = base64gzip(file("${path.module}/initdb/00-init.sh"))
    appdb_sql         = base64gzip(file("${path.module}/initdb/sql/appdb.sql"))
    analyticsdb_sql   = base64gzip(file("${path.module}/initdb/sql/analyticsdb.sql"))
    load_data_sh      = base64gzip(file("${path.module}/scripts/load-data.sh"))
    size_report_sh    = base64gzip(file("${path.module}/scripts/size-report.sh"))
  })
}

resource "stackit_server" "source" {
  project_id        = stackit_resourcemanager_project.source.project_id
  name              = var.server_name
  machine_type      = var.machine_type
  availability_zone = var.availability_zone
  keypair_name      = stackit_key_pair.admin.name

  boot_volume = {
    size                  = var.disk_size_gb
    source_type           = "image"
    source_id             = local.image_id
    delete_on_termination = true
  }

  network_interfaces = [stackit_network_interface.source.network_interface_id]
  user_data          = local.user_data
  labels             = var.labels

  depends_on = [
    stackit_security_group_rule.egress,
    stackit_public_ip.source,
  ]
}
