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

resource "stackit_alb_waf_managed_rule_set" "crs" {
  project_id = stackit_resourcemanager_project.this.project_id
  name       = "alb-demo-crs"
  type       = "TYPE_OWASP_CRS"
}

resource "stackit_alb_waf_custom_rule_group" "rules" {
  project_id = stackit_resourcemanager_project.this.project_id
  name       = "alb-demo-rules"

  rules = [
    {
      description = "Block based on a specific query parameter (?waf_test=trigger)"
      conditions = [
        {
          variable = {
            type  = "VARIABLE_ARGS_GET"
            value = "waf_test"
          }
          operator = {
            type  = "OPERATOR_STREQ"
            value = "trigger"
          }
        }
      ]
      behavior = {
        action  = "ACTION_DENY"
        log     = true
        log_msg = "WAF Test Rule Triggered via Query Parameter"
      }
    },
    {
      description = "Block based on a specific custom header (X-WAF-Test: trigger)"
      conditions = [
        {
          variable = {
            type  = "VARIABLE_REQUEST_HEADERS"
            value = "X-WAF-Test"
          }
          operator = {
            type  = "OPERATOR_STREQ"
            value = "trigger"
          }
        }
      ]
      behavior = {
        action  = "ACTION_DENY"
        log     = true
        log_msg = "WAF Test Rule Triggered via Custom Header"
      }
    }
  ]

  depends_on = [stackit_alb_waf_managed_rule_set.crs]
}

resource "stackit_alb_waf_configuration" "waf" {
  project_id             = stackit_resourcemanager_project.this.project_id
  name                   = "alb-demo-waf"
  managed_rule_set_name  = stackit_alb_waf_managed_rule_set.crs.name
  custom_rule_group_name = stackit_alb_waf_custom_rule_group.rules.name

  depends_on = [stackit_alb_waf_custom_rule_group.rules]
}
