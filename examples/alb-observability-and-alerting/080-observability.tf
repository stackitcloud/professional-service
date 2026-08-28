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

resource "stackit_observability_instance" "this" {
  project_id                             = var.stackit_project_id
  name                                   = "${var.name_prefix}-observability"
  plan_name                              = var.observability_plan_name
  logs_retention_days                    = var.logs_retention_days
  metrics_retention_days                 = var.metrics_retention_days
  metrics_retention_days_5m_downsampling = var.metrics_retention_days
  metrics_retention_days_1h_downsampling = var.metrics_retention_days
  alert_config                           = local.alert_config
}

# Technical credentials of the Observability instance. The load balancer uses
# them for basic authentication against the log and metric push endpoints.
resource "stackit_observability_credential" "alb" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.this.instance_id
  description = "Push credentials for the ${var.name_prefix} load balancer"
}

# The load balancer service stores a copy of the credentials and exposes a
# reference that is set on the load balancer instead of the raw secret.
resource "stackit_loadbalancer_observability_credential" "this" {
  project_id   = var.stackit_project_id
  display_name = "${var.name_prefix}-observability"
  username     = stackit_observability_credential.alb.username
  password     = stackit_observability_credential.alb.password
}
