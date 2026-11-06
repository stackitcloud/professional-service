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
  type = string
}

variable "stackit_region" {
  type        = string
  description = "STACKIT region."
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the service account key file used by Terraform"
  default     = "./keys/stackit-sa.json"
}

variable "name_prefix" {
  type        = string
  description = "Prefix for the names of all resources"
  default     = "iaas-vol-enc"
}

variable "availability_zone" {
  type        = string
  description = "Availability zone of the volume and the server"
  default     = "eu01-3"
}

variable "kms_role" {
  type        = string
  description = "Project role of the service account that IaaS uses to access the KMS key."
  default     = "kms.reader"
}

variable "kek_key_version" {
  type        = number
  description = "Version of the KMS key used for the volume"
  default     = 1
}

variable "image_name_regex" {
  type        = string
  description = "Regular expression for the name of the image"
  default     = "^Windows Server 2025"
}

variable "volume_size" {
  type        = number
  description = "Size of the encrypted boot volume in GB"
  default     = 64
}

variable "volume_performance_class" {
  type        = string
  description = "Performance class of the encrypted volume"
  default     = "storage_premium_perf6"
}

variable "machine_type" {
  type        = string
  description = "Machine type"
  default     = "g2i.4"
}

variable "network_cidr" {
  type        = string
  description = "IPv4 prefix of the network"
  default     = "10.0.112.0/24"
}

variable "network_nameservers" {
  type        = list(string)
  description = "DNS servers of the network"
  default     = ["1.1.1.1", "9.9.9.9"]
}
