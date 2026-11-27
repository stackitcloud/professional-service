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

# A firewall rule needs addresses, PostgreSQL Flex hands out a DNS name.
# Resolved here at plan time, the way a firewall team would do it by hand.
data "dns_a_record_set" "target" {
  host = local.target.host
}

# The one opening the migration needs: source VM -> PostgreSQL Flex, TCP 5432.
# It stands for the change request a firewall team would carry out.
resource "stackit_security_group_rule" "egress_postgres_flex" {
  for_each = toset(data.dns_a_record_set.target.addrs)

  project_id        = local.source.project_id
  security_group_id = local.source.security_group_id
  description       = "Migration: PostgreSQL Flex ${local.target.instance_id}"
  direction         = "egress"
  ether_type        = "IPv4"
  ip_range          = "${each.key}/32"
  protocol = {
    name = "tcp"
  }
  port_range = {
    min = local.target.port
    max = local.target.port
  }
}
