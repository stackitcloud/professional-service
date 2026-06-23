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

#
# Custom User Settings
#

# STACKIT Availability Zone
variable "zone" {
  type        = string
  description = "Availability Zone"
  default     = "eu01-3"
}

# STACKIT VM Flavor
variable "flavor" {
  type        = string
  description = "Flavor ID"
  default     = "g2i.4"
}

# Local VPC Subnet to create Network
variable "LOCAL_SUBNET" {
  type        = string
  description = ""
  default     = "10.10.0.0/24"
}

# STACKIT ProjectID
variable "STACKIT_PROJECT_ID" {
  type        = string
  description = "STACKIT Project ID"
  default     = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}

variable "STACKIT_ORG_ID" {
  type        = string
  description = "STACKIT Org ID"
  default     = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
