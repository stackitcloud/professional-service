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
  alert_config = {
    route = {
      receiver        = "EmailStackit",
      repeat_interval = "1m"
    }
    receivers = [
      {
        name = "EmailStackit",
        email_configs = [
          {
            to = var.alert_email
          }
        ]
      }
    ]
  }
}

resource "stackit_observability_instance" "example" {
  project_id   = var.stackit_project_id
  name         = var.cluster_name
  plan_name    = var.observability_plan_name
  alert_config = local.alert_config
}

resource "stackit_observability_credential" "example" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.example.instance_id
}
