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
  description = "The STACKIT project ID to deploy resources into."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$", var.stackit_project_id))
    error_message = "The stackit_project_id must be a valid UUID."
  }
}

variable "stackit_region" {
  description = "The STACKIT region to deploy resources into."
  type        = string
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  description = "Path to the STACKIT service account key JSON file used for provider authentication."
  type        = string
}

variable "name_prefix" {
  description = "Prefix applied to the names of all resources. Lowercase letters, digits and hyphens only."
  type        = string
  default     = "alb-obs"

  validation {
    condition     = can(regex("^[a-z0-9]+(-[a-z0-9]+)*$", var.name_prefix)) && length(var.name_prefix) <= 20
    error_message = "The name_prefix must be 1-20 characters of lowercase letters, digits and single hyphens, starting and ending with a letter or digit."
  }
}

variable "network_cidr" {
  description = "IPv4 prefix of the private network that hosts the backends and the load balancer."
  type        = string
  default     = "10.20.0.0/24"

  validation {
    condition     = can(cidrnetmask(var.network_cidr))
    error_message = "The network_cidr must be a valid IPv4 CIDR, e.g. 10.20.0.0/24."
  }
}

variable "availability_zones" {
  description = "Availability zones for the backend VMs. One VM is created per zone."
  type        = list(string)
  default     = ["eu01-1", "eu01-2"]

  validation {
    condition     = length(var.availability_zones) >= 1 && length(var.availability_zones) <= 3
    error_message = "Provide between one and three availability zones."
  }
}

variable "machine_type" {
  description = "Machine type of the backend VMs."
  type        = string
  default     = "c2i.1"
}

variable "image_name" {
  description = "Name of the boot image for the backend VMs, resolved via the stackit_image_v2 data source. The image must ship python3."
  type        = string
  default     = "Debian 12"
}

variable "boot_volume_size_gb" {
  description = "Boot volume size of each backend VM in GB."
  type        = number
  default     = 20
}

variable "alb_plan_id" {
  description = "Service plan of the Application Load Balancer. p10 is the smallest plan."
  type        = string
  default     = "p10"
}

variable "alb_allowed_source_ranges" {
  description = "Source CIDRs that may reach the load balancer listeners."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "observability_plan_name" {
  description = "Service plan of the Observability instance. Logs and log alerts require an Observability-* plan, not an Observability-Monitoring-* plan. The plan also caps the metric samples per minute the load balancer may push; choose a larger plan if metrics arrive with gaps."
  type        = string
  default     = "Observability-Starter-EU01"
}

variable "logs_retention_days" {
  description = "Retention of the load balancer logs in the Observability instance."
  type        = number
  default     = 7
}

variable "metrics_retention_days" {
  description = "Retention of the load balancer metrics in the Observability instance."
  type        = number
  default     = 90
}

variable "alert_email" {
  description = "Email address that receives alert notifications. Leave unset to deploy the alert rules without a notification receiver."
  type        = string
  default     = null
}

variable "alert_webhook_url" {
  description = "Webhook URL that receives alert notifications. Leave unset to deploy the alert rules without a notification receiver."
  type        = string
  default     = null
  sensitive   = true
}

variable "alert_traffic_min_bytes_per_second" {
  description = "Throughput in bytes per second that the current throughput (spike alert) or the throughput of the previous hour (drop alert) must exceed before the traffic alerts fire, so that idle load balancers do not alert."
  type        = number
  default     = 100000
}

variable "alert_waf_blocks_per_5m" {
  description = "Number of WAF-blocked requests within five minutes above which the WAF alert fires."
  type        = number
  default     = 10
}
