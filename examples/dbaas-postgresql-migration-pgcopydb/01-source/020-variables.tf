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

variable "stackit_parent_container_id" {
  type        = string
  description = "Organization or folder the source project is created in"
}

variable "stackit_admin_email" {
  type        = string
  description = "Email address that becomes the owner of the created project"
}

variable "stackit_region" {
  type    = string
  default = "eu01"
}

variable "stackit_service_account_key_path" {
  type    = string
  default = null
}


variable "project_name" {
  type    = string
  default = "pg-migration-source"
}

variable "network_prefix" {
  type        = string
  description = "IPv4 prefix of the source network"
  default     = "10.20.0.0/24"
}

variable "nameservers" {
  type    = list(string)
  default = ["1.1.1.1", "9.9.9.9"]
}


variable "server_name" {
  type        = string
  description = "Name of the source VM"
  default     = "pg-source"
}

variable "machine_type" {
  type    = string
  default = "g2i.2"
}

variable "availability_zone" {
  type    = string
  default = "eu01-1"
}

variable "disk_size_gb" {
  type    = number
  default = 50
}

variable "image_name" {
  type    = string
  default = "Ubuntu 24.04"
}

variable "ssh_public_key_path" {
  type = string
}


variable "postgres_image" {
  type        = string
  description = "Container image of the source DBMS. The major version must not be newer than the one of the target."
  default     = "docker.io/library/postgres:18-bookworm"
}

variable "labels" {
  type        = map(string)
  description = "Labels attached to all resources"
  default = {
    example = "dbaas-postgresql-migration-pgcopydb"
  }
}
