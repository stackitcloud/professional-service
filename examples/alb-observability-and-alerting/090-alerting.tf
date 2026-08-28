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

# Metric based rules (PromQL) evaluated by the Observability instance. The
# load balancer exports health check results, connection counts and bytes per
# target pool, but no request counts, status codes or latencies.
resource "stackit_observability_alertgroup" "alb" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.this.instance_id
  name        = "${var.name_prefix}-alb"
  interval    = "60s"

  rules = [
    {
      record     = "alb:healthy_targets:min"
      expression = "min by (stackit_lb_name, envoy_cluster_name) (lb_healthcheck_targets_healthy${local.lb_selector})"
    },
    {
      record     = "alb:throughput_bytes_per_second:rate5m"
      expression = "sum by (stackit_lb_name, envoy_cluster_name) (rate(lb_target_pool_cx_rx_bytes_total${local.lb_selector}[5m]) + rate(lb_target_pool_cx_tx_bytes_total${local.lb_selector}[5m]))"
    },
    {
      alert      = "AlbTargetPoolDegraded"
      expression = "min by (stackit_lb_name, envoy_cluster_name) (lb_healthcheck_targets_healthy${local.lb_selector}) < ${length(local.backends)} and max by (stackit_lb_name, envoy_cluster_name) (lb_healthcheck_targets_healthy${local.lb_selector}) > 0"
      for        = "1m"
      labels = {
        severity = "warning"
      }
      annotations = {
        summary     = "At least one backend of the target pool fails its health check"
        description = "Only {{ $value }} of ${length(local.backends)} backends of {{ $labels.envoy_cluster_name }} pass the active health check on the load balancer instance with the fewest healthy backends."
      }
    },
    {
      alert      = "AlbTargetPoolUnhealthy"
      expression = "max by (stackit_lb_name, envoy_cluster_name) (lb_healthcheck_targets_healthy${local.lb_selector}) == 0"
      for        = "1m"
      labels = {
        severity = "critical"
      }
      annotations = {
        summary     = "No backend of the target pool passes its health check"
        description = "No backend of {{ $labels.envoy_cluster_name }} passes the active health check any more."
      }
    },
    {
      alert      = "AlbHealthCheckFailures"
      expression = "sum by (stackit_lb_name, envoy_cluster_name) (rate(lb_healthcheck_targets_failures_total${local.lb_selector}[5m])) > 0"
      for        = "5m"
      labels = {
        severity = "warning"
      }
      annotations = {
        summary     = "Health checks of the target pool keep failing"
        description = "{{ $value | printf \"%.2f\" }} health checks per second fail for {{ $labels.envoy_cluster_name }}. A backend is down or answers /healthz with an error."
      }
    },
    {
      alert      = "AlbTrafficSpike"
      expression = "sum by (stackit_lb_name) (rate(lb_target_pool_cx_rx_bytes_total${local.lb_selector}[5m]) + rate(lb_target_pool_cx_tx_bytes_total${local.lb_selector}[5m])) > 3 * sum by (stackit_lb_name) (rate(lb_target_pool_cx_rx_bytes_total${local.lb_selector}[1h] offset 5m) + rate(lb_target_pool_cx_tx_bytes_total${local.lb_selector}[1h] offset 5m)) and sum by (stackit_lb_name) (rate(lb_target_pool_cx_rx_bytes_total${local.lb_selector}[5m]) + rate(lb_target_pool_cx_tx_bytes_total${local.lb_selector}[5m])) > ${var.alert_traffic_min_bytes_per_second}"
      for        = "5m"
      labels = {
        severity = "warning"
      }
      annotations = {
        summary     = "Backend throughput is more than three times the throughput of the previous hour"
        description = "The load balancer currently exchanges {{ $value | humanize }}B/s with its backends."
      }
    },
    {
      alert      = "AlbTrafficDrop"
      expression = "sum by (stackit_lb_name) (rate(lb_target_pool_cx_rx_bytes_total${local.lb_selector}[5m]) + rate(lb_target_pool_cx_tx_bytes_total${local.lb_selector}[5m])) < 0.2 * sum by (stackit_lb_name) (rate(lb_target_pool_cx_rx_bytes_total${local.lb_selector}[1h] offset 5m) + rate(lb_target_pool_cx_tx_bytes_total${local.lb_selector}[1h] offset 5m)) and sum by (stackit_lb_name) (rate(lb_target_pool_cx_rx_bytes_total${local.lb_selector}[1h] offset 5m) + rate(lb_target_pool_cx_tx_bytes_total${local.lb_selector}[1h] offset 5m)) > ${var.alert_traffic_min_bytes_per_second}"
      for        = "5m"
      labels = {
        severity = "warning"
      }
      annotations = {
        summary     = "Backend throughput dropped below 20 % of the throughput of the previous hour"
        description = "The load balancer currently exchanges {{ $value | humanize }}B/s with its backends."
      }
    },
    {
      alert      = "AlbMetricsAbsent"
      expression = "absent(lb_healthcheck_targets_healthy{stackit_lb_name=\"${local.alb_name}\"})"
      for        = "10m"
      labels = {
        severity = "warning"
      }
      annotations = {
        summary     = "No metrics of the load balancer have been received for ten minutes"
        description = "The Observability instance receives no metrics from the load balancer. Check the errors attribute of the load balancer and the sample limit of the Observability plan."
      }
    },
    {
      alert      = "ObservabilitySamplesRejected"
      expression = "increase(instance_remote_write_samples_rejected_total[10m]) > 0"
      for        = "0s"
      labels = {
        severity = "warning"
      }
      annotations = {
        summary     = "The Observability instance rejected metric samples"
        description = "{{ $value | humanize }} samples were rejected during the last ten minutes, usually because the sample limit of the plan was exceeded."
      }
    },
  ]
}

# Log based rules (LogQL) evaluated against the WAF log stream of the load balancer.
resource "stackit_observability_logalertgroup" "waf" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.this.instance_id
  name        = "${var.name_prefix}-waf"
  interval    = "60s"

  rules = [
    {
      alert      = "AlbWafBlockRateSpike"
      expression = "sum(count_over_time({component=\"waf\", stackit_lb_name=\"${local.alb_name}\"} |= \"Access denied\" [5m])) > ${var.alert_waf_blocks_per_5m}"
      for        = "1m"
      labels = {
        severity = "warning"
      }
      annotations = {
        summary     = "The WAF blocked more than ${var.alert_waf_blocks_per_5m} requests within five minutes"
        description = "{{ $value }} requests were denied by the WAF during the last five minutes. Check the WAF log stream for the rule IDs and client addresses."
      }
    },
    {
      alert      = "AlbWafCustomRuleTriggered"
      expression = "sum(count_over_time({component=\"waf\", stackit_lb_name=\"${local.alb_name}\"} |= \"Custom rule: X-Waf-Demo header\" [5m])) > 0"
      for        = "0s"
      labels = {
        severity = "info"
      }
      annotations = {
        summary     = "The custom WAF rule denied a request"
        description = "{{ $value }} requests matched the custom deny rule during the last five minutes."
      }
    },
  ]
}
