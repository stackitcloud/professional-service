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

variable "stackit_project_id" {
  description = "STACKIT project for SKE & Observability"
  type        = string
}

variable "stackit_region" {
  description = "STACKIT region."
  type        = string
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  description = "Path to the STACKIT service account key JSON file."
  type        = string
}

variable "network_ipv4_prefix" {
  type        = string
  description = "IPv4 prefix of the SKE node network."
}

variable "cluster_name" {
  description = "Name of the SKE cluster."
  type        = string
  default     = "example"
}

variable "node_pool_machine_type" {
  description = "Machine type of the node pool."
  type        = string
  default     = "c2i.4"
}

variable "node_pool_minimum" {
  description = "Minimum number of nodes in the node pool."
  type        = number
  default     = 3
}

variable "node_pool_maximum" {
  description = "Maximum number of nodes in the node pool."
  type        = number
  default     = 9
}

variable "node_pool_availability_zones" {
  description = "Availability zones of the node pool."
  type        = list(string)
  default     = ["eu01-2"]
}

variable "observability_plan_name" {
  description = "Plan of the Observability instance."
  type        = string
  default     = "Observability-Large-EU01"
}

variable "alert_email" {
  description = "E-Mail Address that receives the metric / log alert notifications."
  type        = string
}

variable "audit_logs_enabled" {
  description = "Ships the kube-apiserver audit logs through the Telemetry Router"
  type        = bool
  default     = true
}

variable "telemetry_link_token_version" {
  type    = number
  default = 1
}

variable "collector_namespace" {
  type    = string
  default = "opentelemetry"
}

variable "otel_chart_version" {
  description = "Version of the open-telemetry/opentelemetry-collector Helm chart."
  type        = string
  default     = "0.173.1"
}

variable "otel_operator_chart_version" {
  description = "Version of the open-telemetry/opentelemetry-operator Helm chart."
  type        = string
  default     = "0.123.1"
}

variable "operator_namespace" {
  type    = string
  default = "opentelemetry-operator"
}

variable "workload_image" {
  description = "Base image of the demo workload"
  type        = string
  default     = "python:3.14-slim"
}

variable "workload_namespace" {
  type    = string
  default = "example"
}
