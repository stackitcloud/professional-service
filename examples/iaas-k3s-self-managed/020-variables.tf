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
  description = "The STACKIT project ID to deploy resources into."
}

variable "stackit_region" {
  type        = string
  default     = "eu01"
  description = "The STACKIT region to deploy resources into."
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the STACKIT service account key JSON file used for Terraform provider authentication."
}

variable "cluster_name" {
  type        = string
  default     = "k3s-cluster"
  description = "Base name used for all cluster resources (servers, network, security group)."
}

variable "k3s_version" {
  type        = string
  default     = "v1.36.3+k3s1"
  description = "k3s version to install on all nodes (e.g. 'v1.36.3+k3s1'). Must match the cloud-provider-stackit minor version. See https://github.com/k3s-io/k3s/releases."
}

variable "cp_machine_type" {
  type        = string
  default     = "c2i.4"
  description = "STACKIT machine type for the 3 control plane nodes (4 vCPU, 8 GB RAM)."
}

variable "worker_machine_type" {
  type        = string
  default     = "c2i.4"
  description = "STACKIT machine type for the 3 worker nodes."
}

variable "disk_size_gb" {
  type        = number
  default     = 50
  description = "Boot disk size in GiB for all nodes."
}

variable "ssh_public_key_path" {
  type        = string
  default     = "~/.ssh/id_rsa.pub"
  description = "Path to the SSH public key injected into every node for access."
}

variable "ssh_private_key_path" {
  type        = string
  default     = "~/.ssh/id_rsa"
  description = "Path to the SSH private key used by Terraform to bootstrap the cluster. Not stored in state."
  sensitive   = true
}

variable "ccm_repo_url" {
  type        = string
  default     = "https://github.com/stackitcloud/cloud-provider-stackit"
  description = "URL of the cloud-provider-stackit repository (CCM + CSI driver)."
}

variable "ccm_branch" {
  type        = string
  default     = "release-v1.36"
  description = "Branch of cloud-provider-stackit to deploy from. Must align with the k8s minor version (e.g. 'release-v1.36' for k3s v1.36.x)."
}

variable "alb_controller_image" {
  type        = string
  default     = "ghcr.io/stackitcloud/application-load-balancer-controller:v0.2.0"
  description = "Container image for the STACKIT ALB Ingress controller. See https://github.com/stackitcloud/application-load-balancer-controller/releases."
}

locals {
  availability_zones = ["eu01-1", "eu01-2", "eu01-3"]

  cp_names     = [for i in range(3) : "${var.cluster_name}-cp-${i + 1}"]
  worker_names = [for i in range(3) : "${var.cluster_name}-worker-${i + 1}"]
}
