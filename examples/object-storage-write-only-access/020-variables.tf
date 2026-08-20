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
  description = "The STACKIT Project ID where the Object Storage resources will be created"
  type        = string
}

variable "stackit_service_account_key_path" {
  description = "Path to the STACKIT service account key JSON file"
  type        = string
  default     = ""
}

variable "bucket_name" {
  description = "Name of the Object Storage bucket"
  type        = string
  default     = "write-only-locked-bucket"
}

variable "owner_email" {
  description = "Email address of the project owner for the credentials sub-project"
  type        = string
}

variable "credentials_project_name" {
  description = "Name of the sub-project created for credentials isolation"
  type        = string
  default     = "s3-credentials-project"
}
