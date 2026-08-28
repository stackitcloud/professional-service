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
  labels = {
    example    = "alb-observability-and-alerting"
    managed-by = "terraform"
  }

  # One backend VM per availability zone, keyed by a two-digit index.
  backends = { for idx, az in var.availability_zones : format("%02d", idx + 1) => az }

  backend_port     = 8080
  target_pool_name = "${var.name_prefix}-backends"
  alb_name         = "${var.name_prefix}-alb"

  # Label selector that scopes the PromQL expressions to this load balancer.
  # The load balancer reports one Envoy cluster per target pool and listener
  # plus an internal xds_cluster, which is excluded.
  lb_selector = "{stackit_lb_name=\"${local.alb_name}\", envoy_cluster_name!=\"xds_cluster\"}"

  # Alertmanager configuration of the Observability instance. Rules are always
  # deployed; notifications are only routed when at least one receiver is set.
  # The webhook URL is a sensitive variable; comparing it with null would mark
  # the whole alert_config as sensitive and hide it from the plan output, so
  # only the presence flag is unmasked. The URL itself stays redacted.
  webhook_set      = nonsensitive(var.alert_webhook_url != null)
  alerting_enabled = var.alert_email != null || local.webhook_set

  alert_config = local.alerting_enabled ? {
    receivers = [
      {
        name             = "default"
        email_configs    = var.alert_email != null ? [{ to = var.alert_email }] : null
        webhooks_configs = local.webhook_set ? [{ url = var.alert_webhook_url }] : null
      }
    ]
    route = {
      receiver        = "default"
      group_by        = ["alertname"]
      group_wait      = "30s"
      group_interval  = "5m"
      repeat_interval = "4h"
    }
  } : null
}
