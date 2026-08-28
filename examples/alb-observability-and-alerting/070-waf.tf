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

# Minimal WAF setup so that blocked requests show up in the logs. See
# examples/iaas-cross-az-layer7-loadbalancer-waf for the full WAF example.

resource "stackit_alb_waf_managed_rule_set" "crs" {
  project_id = var.stackit_project_id
  name       = "${var.name_prefix}-crs"
  type       = "TYPE_OWASP_CRS"
}

resource "stackit_alb_waf_custom_rule_group" "this" {
  project_id = var.stackit_project_id
  name       = "${var.name_prefix}-custom-rules"

  rules = [
    {
      description = "Deny requests that carry the header X-Waf-Demo: block"
      conditions = [
        {
          variable = {
            type  = "VARIABLE_REQUEST_HEADERS"
            value = "X-Waf-Demo"
          }
          operator = {
            type  = "OPERATOR_STREQ"
            value = "block"
          }
        }
      ]
      behavior = {
        action  = "ACTION_DENY"
        log     = true
        log_msg = "Custom rule: X-Waf-Demo header"
      }
    }
  ]

  depends_on = [stackit_alb_waf_managed_rule_set.crs]
}

resource "stackit_alb_waf_configuration" "this" {
  project_id             = var.stackit_project_id
  name                   = "${var.name_prefix}-waf"
  managed_rule_set_name  = stackit_alb_waf_managed_rule_set.crs.name
  custom_rule_group_name = stackit_alb_waf_custom_rule_group.this.name
  labels                 = local.labels

  depends_on = [stackit_alb_waf_custom_rule_group.this]
}
