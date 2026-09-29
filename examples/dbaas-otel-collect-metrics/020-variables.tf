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
  description = "STACKIT project that holds the cluster, the database and the Observability instance"
}

variable "stackit_region" {
  type        = string
  description = "STACKIT region"
  default     = "eu01"
}

variable "observability_plan_name" {
  type        = string
  description = "Plan of the Observability instance, for example Observability-Starter-EU01 for a smaller one"
  default     = "Observability-Large-EU01"
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the service account key file. Unset falls back to the STACKIT_SERVICE_ACCOUNT_KEY_PATH environment variable, then to that entry in the credentials file ($HOME/.stackit/credentials.json, or STACKIT_CREDENTIALS_PATH)."
  default     = null
}
