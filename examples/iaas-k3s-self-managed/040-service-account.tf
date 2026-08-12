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

# Single service account used by CCM, CSI driver, and ALB controller.
resource "stackit_service_account" "cluster" {
  project_id = var.stackit_project_id
  name       = var.cluster_name
}

resource "stackit_service_account_key" "cluster" {
  project_id            = var.stackit_project_id
  service_account_email = stackit_service_account.cluster.email
}

# CCM needs nlb.admin to manage Network Load Balancers (Service type=LoadBalancer).
resource "stackit_authorization_project_role_assignment" "nlb_admin" {
  resource_id = var.stackit_project_id
  role        = "nlb.admin"
  subject     = stackit_service_account.cluster.email
}

# CSI driver needs blockstorage.admin to manage persistent volumes.
resource "stackit_authorization_project_role_assignment" "blockstorage_admin" {
  resource_id = var.stackit_project_id
  role        = "blockstorage.admin"
  subject     = stackit_service_account.cluster.email
}

# ALB controller needs alb.admin to manage Application Load Balancers (Ingress).
resource "stackit_authorization_project_role_assignment" "alb_admin" {
  resource_id = var.stackit_project_id
  role        = "alb.admin"
  subject     = stackit_service_account.cluster.email
}

# DNS integration for automatic DNS record management.
resource "stackit_authorization_project_role_assignment" "dns_admin" {
  resource_id = var.stackit_project_id
  role        = "dns.admin"
  subject     = stackit_service_account.cluster.email
}
