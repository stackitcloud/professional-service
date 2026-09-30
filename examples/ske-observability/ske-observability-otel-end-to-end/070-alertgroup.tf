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

resource "stackit_observability_alertgroup" "metrics" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.example.instance_id
  name        = "MetricAlerts"
  interval    = "1m"
  rules = [
    {
      # Control plane: via the SKE observability extension
      alert      = "APIServerErrorRateHigh"
      expression = "sum(rate(apiserver_request_total{code=~\"5..\"}[5m])) / sum(rate(apiserver_request_total[5m])) > 0.05"
      for        = "10m"
      labels = {
        severity = "critical"
        source   = "ske-extension"
      }
      annotations = {
        summary     = "kube-apiserver answers more than 5% of requests with 5xx"
        description = "The control plane is managed by STACKIT. If this persists and you experience issues, open a support ticket."
      }
    },
    {
      # Cluster DNS, via prometheus/coredns on the gateway. Example of custom metrics.
      alert      = "CoreDNSServfailRateHigh"
      expression = "sum(rate(coredns_dns_responses_total{rcode=\"SERVFAIL\"}[5m])) / sum(rate(coredns_dns_responses_total[5m])) > 0.05"
      for        = "10m"
      labels = {
        severity = "warning"
        source   = "otel-gateway"
      }
      annotations = {
        summary     = "CoreDNS fails more than 5% of lookups"
        description = "Check upstream resolvers and the coredns Pods in kube-system."
      }
    },
    {
      # Node, via kubeletstats on the agent
      alert      = "NodeFilesystemAlmostFull"
      expression = "k8s_node_filesystem_available_bytes / k8s_node_filesystem_capacity_bytes < 0.10"
      for        = "15m"
      labels = {
        severity = "warning"
        source   = "otel-agent"
      }
      annotations = {
        summary     = "Node filesystem has less than 10% space left"
        description = "{{ $labels.k8s_node_name }} is running out of disk. Full disks evict Pods."
      }
    },
    {
      # Container, via kubeletstats (usage) and k8s_cluster (limit)
      alert      = "ContainerMemoryNearLimit"
      expression = "container_memory_working_set_bytes{k8s_container_name!=\"\"} / on(k8s_namespace_name, k8s_pod_name, k8s_container_name) k8s_container_memory_limit_bytes > 0.9"
      for        = "10m"
      labels = {
        severity = "warning"
        source   = "otel-agent"
      }
      annotations = {
        summary     = "Container uses more than 90% of its memory limit"
        description = "{{ $labels.k8s_container_name }} in {{ $labels.k8s_namespace_name }}/{{ $labels.k8s_pod_name }} is about to be OOM-killed."
      }
    },
    {
      # Kubernetes object state, via k8s_cluster on the gateway.
      alert      = "DeploymentPodsMissing"
      expression = "k8s_deployment_available < k8s_deployment_desired"
      for        = "5m"
      labels = {
        severity = "warning"
        source   = "otel-gateway"
      }
      annotations = {
        summary     = "Deployment has fewer ready Pods than desired"
        description = "{{ $labels.k8s_deployment_name }} in {{ $labels.k8s_namespace_name }} is short of Pods."
      }
    },
    {
      # Application traces, via the spanmetrics connector on the gateway.
      # The demo fails 20% of its requests, so this fires on purpose.
      alert      = "ServiceErrorRateHigh"
      expression = "sum by (service_name) (rate(traces_span_metrics_calls_total{span_kind=\"SPAN_KIND_SERVER\", status_code=\"STATUS_CODE_ERROR\"}[5m])) / sum by (service_name) (rate(traces_span_metrics_calls_total{span_kind=\"SPAN_KIND_SERVER\"}[5m])) > 0.10"
      for        = "5m"
      labels = {
        severity = "critical"
        source   = "otel-gateway"
      }
      annotations = {
        summary     = "More than 10% of server spans end in an error"
        description = "{{ $labels.service_name }} fails requests. Open Tempo and search for status = error."
      }
    },
  ]
}

locals {
  log_alert_rules = [
    {
      # Container logs, via filelog on the agent.
      # The demo logs these lines on purpose, so this fires.
      alert      = "ApplicationErrorsInLogs"
      expression = "sum by (service_name) (count_over_time({service_name=\"logger\"} |= \"Simulated error message\" [5m])) > 0"
      for        = "1m"
      labels = {
        severity = "critical"
        source   = "otel-agent"
      }
      annotations = {
        summary     = "Application is logging errors"
        description = "{{ $labels.service_name }} wrote error lines in the last five minutes."
      }
    },
    {
      # Kubernetes events, via k8sobjects on the gateway
      alert      = "KubernetesWarningEvents"
      expression = "sum(count_over_time({service_name=\"kubernetes-events\"} |~ \"BackOff|FailedScheduling|OOMKilling|FailedMount\" [5m])) > 0"
      for        = "5m"
      labels = {
        severity = "warning"
        source   = "otel-gateway"
      }
      annotations = {
        summary     = "Pods crash, cannot be scheduled or cannot mount volumes"
        description = "Query {service_name=\"kubernetes-events\"} in Grafana for the affected objects."
      }
    },
  ]

  # Every read of a Secret by a human identity. Controllers and Gardener
  # components are filtered out; they read Secrets all the time.
  audit_alert_rules = [
    {
      # kube-apiserver audit logs, via the Telemetry Router
      alert      = "SecretReadByHuman"
      expression = "sum by (user, verb) (count_over_time({service_name=\"ske\"} | json user=\"user.username\", verb=\"verb\", resource=\"objectRef.resource\", stage=\"stage\" | resource=\"secrets\" | stage=\"ResponseComplete\" | verb=~\"get|list\" | user!~`system:.*` | user!~`gardener\\.cloud:.*` [5m])) > 0"
      for        = "1m"
      labels = {
        severity = "info"
        source   = "telemetry-router"
      }
      annotations = {
        summary     = "A human identity read Secrets"
        description = "{{ $labels.user }} ran {{ $labels.verb }} on secrets. terraform plan does this too."
      }
    },
  ]
}

resource "stackit_observability_logalertgroup" "logs" {
  project_id  = var.stackit_project_id
  instance_id = stackit_observability_instance.example.instance_id
  name        = "LogAlerts"
  interval    = "1m"
  rules       = var.audit_logs_enabled ? concat(local.log_alert_rules, local.audit_alert_rules) : local.log_alert_rules
}
