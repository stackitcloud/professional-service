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


# -- variables (values are passed in from the root module)
variable "organization_id" {
  type        = string
  description = "STACKIT organization ID"
}

variable "default_region" {
  type        = string
  description = "Region for the network area"
}

variable "project_name" {
  type        = string
  description = "Name of the project created in the network area"
}

variable "owner_email" {
  type        = string
  description = "Owner e-mail of the project"
}

variable "project_deletion_wait" {
  type        = string
  description = "Wait time on destroy between deleting the project and the network area region"
  default     = "180s"
}
