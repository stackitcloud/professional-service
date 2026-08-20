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
  type        = string
  description = "STACKIT project ID."
}

variable "stackit_region" {
  type        = string
  default     = "eu01"
  description = "STACKIT region."
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the STACKIT service account key JSON file."
}

variable "stec_plan_id" {
  type        = string
  description = "STEC plan ID. Retrieve with: stackit beta edge-cloud plans list"
}

variable "stec_instance_name" {
  type        = string
  default     = "test"
  description = "Display name for the STEC management plane instance."
}

variable "talos_version" {
  type        = string
  default     = "v1.13.5-stackit.v1.7.16"
  description = "Talos version. Check: https://image-factory.edge.eu01.stackit.cloud/versions"
}

variable "kubernetes_version" {
  type        = string
  default     = "v1.33.0"
  description = "Kubernetes version for the edge cluster."
}

variable "cluster_name" {
  type        = string
  default     = "edge-test-cluster"
  description = "Name of the EdgeCluster resource (RFC 1034)."
}

variable "cp_count" {
  type        = number
  default     = 3
  description = "Number of control plane nodes."
}

variable "worker_count" {
  type        = number
  default     = 3
  description = "Number of worker nodes."
}

variable "cp_machine_type" {
  type        = string
  default     = "c2i.4"
  description = "Machine type for control plane nodes."
}

variable "worker_machine_type" {
  type        = string
  default     = "c2i.8"
  description = "Machine type for worker nodes."
}

variable "disk_size_gb" {
  type        = number
  default     = 100
  description = "Boot disk size in GiB (minimum 32)."

  validation {
    condition     = var.disk_size_gb >= 32
    error_message = "Disk size must be at least 32 GiB."
  }
}

variable "network_ipv4_prefix" {
  type        = string
  default     = "10.0.10.0/24"
  description = "IPv4 prefix of the cluster network (must be a subnet of the SNA network range)."

  validation {
    condition     = can(cidrhost(var.network_ipv4_prefix, 0))
    error_message = "Must be a valid IPv4 CIDR, e.g. 10.0.10.0/24."
  }
}

variable "availability_zone" {
  type        = string
  default     = "eu01-1"
  description = "Availability zone for all nodes."
}

locals {
  common_labels = {
    "managed-by" = "terraform"
    "example"    = "iaas-edge-k8s-cluster"
  }
}
