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


# --  variables
variable "organization_id" {
  type        = string
  description = "STACKIT organization ID (required)"
  default     = "" # set your organization ID here

  validation {
    condition     = can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", var.organization_id))
    error_message = "Set organization_id in 020-variables.tf to your STACKIT organization ID (UUID)."
  }
}

variable "owner_email" {
  type        = string
  description = "Owner e-mail of the STACKIT project (required)"
  default     = "" # set your e-mail here

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.owner_email))
    error_message = "Set owner_email in 020-variables.tf to the e-mail address of the project owner."
  }
}

variable "project_name" {
  type        = string
  description = "Name of the STACKIT project that is created in the network area"
  default     = "paloalto-ha"
}

variable "service_account_key_path" {
  type        = string
  description = "Path to the STACKIT service account key JSON (required)"
  default     = "~/.stackit/credentials.json"

  validation {
    condition     = can(jsondecode(file(pathexpand(var.service_account_key_path))))
    error_message = "Set service_account_key_path in 020-variables.tf to an existing STACKIT service account key (JSON file)."
  }
}

variable "default_region" {
  type        = string
  description = "STACKIT region"
  default     = "eu01"
}

variable "image_file" {
  type        = string
  description = "Path to the Palo Alto VM-Series KVM image (qcow2), downloaded from the Palo Alto support portal (required)"
  default     = "./PA-VM-KVM-11.2.5.qcow2"

  validation {
    condition     = fileexists(pathexpand(var.image_file))
    error_message = "Set image_file in 020-variables.tf to the downloaded PA-VM-KVM qcow2 image."
  }
}

variable "image_name" {
  type        = string
  description = "Name of the uploaded image in the project"
  default     = "PA-VM-KVM-11.2.5"
}

variable "flavor" {
  type        = string
  description = "Machine type of the firewalls"
  default     = "m1.2"
}

variable "boot_volume_size" {
  type        = number
  description = "Size of the boot volume of each firewall in GB (the image needs at least 80)"
  default     = 100
}

variable "boot_volume_performance_class" {
  type        = string
  description = "Performance class of the boot volumes"
  default     = "storage_premium_perf2"
}

variable "firewalls" {
  type = map(object({
    availability_zone = string
    host              = number # host part of the fixed IPs in every network, e.g. 10 => 10.220.x.10
  }))
  description = "HA pair: firewall name => availability zone and host part of its IPs"
  default = {
    fw1 = {
      availability_zone = "eu01-1"
      host              = 10
    }
    fw2 = {
      availability_zone = "eu01-2"
      host              = 11
    }
  }

  validation {
    condition     = length(var.firewalls) == 2
    error_message = "A Palo Alto HA pair consists of exactly two firewalls."
  }
}

variable "vip_host" {
  type        = number
  description = "Host part of the VIP in wan_network"
  default     = 30
}

variable "lan_gateway_host" {
  type        = number
  description = "Host part of the LAN IP of the active firewall in lan_network1/2, announced as gateway by DHCP. Must not be the IP of a port"
  default     = 1
}

variable "mgmt_allowed_cidr" {
  type        = string
  description = "Source range allowed to reach the management interfaces (HTTPS, SSH). Restrict to your own IP, e.g. 1.2.3.4/32"
  default     = "0.0.0.0/0"
}
