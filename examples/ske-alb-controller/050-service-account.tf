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

resource "stackit_service_account" "alb_controller" {
  project_id = stackit_resourcemanager_project.this.project_id
  name       = "alb-controller"
}

resource "stackit_authorization_project_custom_role" "alb_writer" {
  resource_id = stackit_resourcemanager_project.this.project_id
  name        = "alb.controller.writer"
  description = "Write access to Application Load Balancers and ALB certificates for the ALB controller"
  permissions = [
    "alb.ping",
    "alb.quota.get",
    "alb.loadbalancer.create",
    "alb.loadbalancer.delete",
    "alb.loadbalancer.get",
    "alb.loadbalancer.list",
    "alb.loadbalancer.update",
    "alb.targetpool.replace",
    "alb.certificateservice.ping",
    "alb.certificateservice.quota.get",
    "alb.certificateservice.certificate.create",
    "alb.certificateservice.certificate.delete",
    "alb.certificateservice.certificate.get",
    "alb.certificateservice.certificate.list",
  ]
}

resource "stackit_authorization_project_role_assignment" "alb_controller" {
  resource_id = stackit_resourcemanager_project.this.project_id
  role        = stackit_authorization_project_custom_role.alb_writer.name
  subject     = stackit_service_account.alb_controller.email
}

resource "time_rotating" "alb_controller_key" {
  rotation_days = 80
}

resource "stackit_service_account_key" "alb_controller" {
  project_id            = stackit_resourcemanager_project.this.project_id
  service_account_email = stackit_service_account.alb_controller.email
  ttl_days              = 90

  rotate_when_changed = {
    rotation = time_rotating.alb_controller_key.id
  }
}
