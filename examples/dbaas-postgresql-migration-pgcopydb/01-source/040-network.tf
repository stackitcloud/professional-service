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

resource "stackit_network" "source" {
  project_id       = stackit_resourcemanager_project.source.project_id
  name             = "${var.server_name}-network"
  ipv4_prefix      = var.network_prefix
  ipv4_nameservers = var.nameservers
  labels           = var.labels
}

resource "stackit_security_group" "firewall" {
  project_id  = stackit_resourcemanager_project.source.project_id
  name        = "${var.server_name}-enterprise-firewall"
  description = "Simulated enterprise firewall in front of the source DBMS"
  stateful    = true
  labels      = var.labels
}

# Remove default egress rules (which allow everything)
# see https://docs.stackit.cloud/de/products/network/core-networking/security-groups/how-tos/minimal-security-groups/
resource "terraform_data" "remove_default_egress" {
  triggers_replace = [stackit_security_group.firewall.security_group_id]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    environment = {
      PROJECT_ID = stackit_resourcemanager_project.source.project_id
      SG_ID      = stackit_security_group.firewall.security_group_id
      REGION     = var.stackit_region
    }
    command = <<-EOT
      set -euo pipefail
      list() {
        stackit security-group rule list --project-id "$PROJECT_ID" --region "$REGION" \
          --security-group-id "$SG_ID" -o json 2>/dev/null \
          | jq -r 'if type == "array" then .[] | select(.direction == "egress") | .id else empty end' 2>/dev/null || true
      }
      for id in $(list); do
        echo "Removing default egress rule $id"
        stackit security-group rule delete "$id" --project-id "$PROJECT_ID" --region "$REGION" \
          --security-group-id "$SG_ID" -y
      done
      left=$(list | wc -l | tr -d ' ')
      if [ "$left" != "0" ]; then
        echo "Still $left egress rule(s) in $SG_ID, the firewall would not be closed." >&2
        exit 1
      fi
    EOT
  }
}

locals {
  sg_project_id = stackit_resourcemanager_project.source.project_id
  sg_id         = stackit_security_group.firewall.security_group_id

  # What a typical enterprise firewall lets a database server do: resolve names,
  # reach package mirrors and registries over HTTP(S), sync the clock...
  egress_rules = merge(
    {
      for ns in var.nameservers : "dns-udp-${ns}" => { protocol = "udp", port = 53, ip_range = "${ns}/32", description = "DNS" }
    },
    {
      for ns in var.nameservers : "dns-tcp-${ns}" => { protocol = "tcp", port = 53, ip_range = "${ns}/32", description = "DNS" }
    },
    {
      dhcp  = { protocol = "udp", port = 67, ip_range = "0.0.0.0/0", description = "DHCP" }
      http  = { protocol = "tcp", port = 80, ip_range = "0.0.0.0/0", description = "Package mirrors, metadata service" }
      https = { protocol = "tcp", port = 443, ip_range = "0.0.0.0/0", description = "Package mirrors, container registries" }
      ntp   = { protocol = "udp", port = 123, ip_range = "0.0.0.0/0", description = "Time sync" }
    },
  )
}

resource "stackit_security_group_rule" "ssh_admin" {
  project_id        = local.sg_project_id
  security_group_id = local.sg_id
  description       = "SSH, key login only"
  direction         = "ingress"
  ether_type        = "IPv4"
  ip_range          = "0.0.0.0/0"
  protocol = {
    name = "tcp"
  }
  port_range = {
    min = 22
    max = 22
  }
}

resource "stackit_security_group_rule" "egress" {
  for_each = local.egress_rules

  project_id        = local.sg_project_id
  security_group_id = local.sg_id
  description       = each.value.description
  direction         = "egress"
  ether_type        = "IPv4"
  ip_range          = each.value.ip_range
  protocol = {
    name = each.value.protocol
  }
  port_range = {
    min = each.value.port
    max = each.value.port
  }

  depends_on = [terraform_data.remove_default_egress]
}
