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

module "test-machine02" {
  source = "../../modules/test-machine"

  project_id        = var.stackit_project_id
  network_id        = stackit_network.network.network_id
  availability_zone = "eu01-2"

  name         = "machine02"
  machine_type = var.jumphost_flavor
  disk_size    = 48

  user_data = templatefile("${path.module}/apache-debug-user.yaml", {})
}
