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

data "terraform_remote_state" "source" {
  backend = "local"
  config = {
    path = var.source_state_path
  }
}

data "terraform_remote_state" "target" {
  backend = "local"
  config = {
    path = var.target_state_path
  }
}

locals {
  source = data.terraform_remote_state.source.outputs
  target = data.terraform_remote_state.target.outputs
}
