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

resource "stackit_service_account" "costguard" {
  project_id = local.project_id
  name       = "costguard"
}

locals {
  reader_permissions = [
    "resource-manager.organization.get",
    "resource-manager.folder.list",
    "resource-manager.folder.get",
    "resource-manager.project.list",
    "resource-manager.project.get",
    "iaas.server.list", "iaas.server.get",
    "iaas.volume.list", "iaas.volume.get",
    "iaas.public-ip.list", "iaas.public-ip.get",
    "iaas.snapshot.list", "iaas.snapshot.get",
    "iaas.nic.list", "iaas.nic.get",
    "iaas.security-group.list", "iaas.security-group.get",
    "iaas.network-area.list",
    "iaas.network-area.project.list",
    "ske.cluster.list",
    "object-storage.bucket.list",
    "nlb.loadbalancer.list",
    "alb.loadbalancer.list",
    "cost-management.billing.get",
  ]
  cleaner_permissions = [
    "iaas.volume.update",
    "iaas.public-ip.update",
    "iaas.server.delete",
    "iaas.volume.delete",
    "iaas.public-ip.delete",
    "iaas.snapshot.delete",
    "iaas.nic.delete",
    "iaas.security-group.delete",
  ]
}

resource "stackit_authorization_organization_custom_role" "reader" {
  resource_id = var.organization_id
  name        = "costguard.reader"
  description = "costguard: read resources and costs (managed by Terraform)"
  permissions = sort(local.reader_permissions)
}

resource "stackit_authorization_organization_role_assignment" "reader" {
  resource_id = var.organization_id
  role        = stackit_authorization_organization_custom_role.reader.name
  subject     = stackit_service_account.costguard.email
}

resource "stackit_authorization_organization_custom_role" "cleaner" {
  count = local.delete_enabled ? 1 : 0

  resource_id = var.organization_id
  name        = "costguard.cleaner"
  description = "costguard: label and delete resources (managed by Terraform)"
  permissions = sort(local.cleaner_permissions)
}

resource "stackit_authorization_organization_role_assignment" "cleaner" {
  count = local.delete_enabled ? 1 : 0

  resource_id = var.organization_id
  role        = stackit_authorization_organization_custom_role.cleaner[0].name
  subject     = stackit_service_account.costguard.email
}
