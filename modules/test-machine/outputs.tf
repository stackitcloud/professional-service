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

output "server_id" {
  value = stackit_server.server.server_id
}

output "server_name" {
  value = stackit_server.server.name
}

output "primary_ip" {
  description = "The primary ipv4 internal address assigned to the interface"
  value       = stackit_network_interface.nic.ipv4
}
