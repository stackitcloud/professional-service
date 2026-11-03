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

# Reads the outputs of 01-source. The count skips that when both values are
# given explicitly.
data "terraform_remote_state" "source" {
  count = var.source_egress_cidrs == null || var.databases == null ? 1 : 0

  backend = "local"
  config = {
    path = var.source_state_path
  }
}

locals {
  source_outputs = one(data.terraform_remote_state.source[*].outputs)

  source_egress_cidrs = coalesce(
    var.source_egress_cidrs,
    try(["${local.source_outputs.public_ip}/32"], null),
  )

  databases = coalesce(
    var.databases,
    try(local.source_outputs.source_databases, null),
  )
}
