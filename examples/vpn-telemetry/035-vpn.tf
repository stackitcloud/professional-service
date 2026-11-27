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

# "onprem" simulates the on-premises network, "stackit" is the STACKIT side.
# They are separate Network Areas, in the same area the projects would
# route directly and never use the VPN.
module "onprem" {
  source                    = "../../modules/stackit-sna-with-debug-machine"
  stackit_org_id            = var.stackit_org_id
  stackit_admin_email       = var.stackit_admin_email
  stackit_project_name      = "${var.name_prefix}-onprem"
  sna_name                  = "${var.name_prefix}-onprem"
  sna_network_range_prefix  = ["10.10.0.0/16"]
  machine_name              = "${var.name_prefix}-onprem"
  machine_network_name      = "${var.name_prefix}-onprem"
  machine_ipv4_prefix       = "10.10.10.0/24"
  machine_availability_zone = "eu01-1"
}

module "stackit" {
  source                    = "../../modules/stackit-sna-with-debug-machine"
  stackit_org_id            = var.stackit_org_id
  stackit_admin_email       = var.stackit_admin_email
  stackit_project_name      = "${var.name_prefix}-stackit"
  sna_name                  = "${var.name_prefix}-stackit"
  sna_network_range_prefix  = ["10.11.0.0/16"]
  machine_name              = "${var.name_prefix}-stackit"
  machine_network_name      = "${var.name_prefix}-stackit"
  machine_ipv4_prefix       = "10.11.11.0/24"
  machine_availability_zone = "eu01-2"
}

resource "stackit_vpn_gateway" "onprem" {
  project_id   = module.onprem.project_id
  display_name = "${var.name_prefix}-onprem"
  plan_id      = "p100"
  routing_type = "BGP_ROUTE_BASED"

  availability_zones = {
    tunnel1 = "eu01-1"
    tunnel2 = "eu01-2"
  }

  network_config = {}

  bgp = {
    local_asn                  = 64512
    override_advertised_routes = ["10.10.0.0/16"]
  }

}

resource "stackit_vpn_gateway" "stackit" {
  project_id   = module.stackit.project_id
  display_name = "${var.name_prefix}-stackit"
  plan_id      = "p100"
  routing_type = "BGP_ROUTE_BASED"

  availability_zones = {
    tunnel1 = "eu01-1"
    tunnel2 = "eu01-2"
  }

  network_config = {}

  bgp = {
    local_asn                  = 64513
    override_advertised_routes = ["10.11.0.0/16"]
  }

  # the link has to exist first, otherwise the creation is not logged
  depends_on = [stackit_telemetrylink.stackit]
}

data "stackit_vpn_gateway_status" "onprem" {
  project_id = module.onprem.project_id
  gateway_id = stackit_vpn_gateway.onprem.gateway_id
}

data "stackit_vpn_gateway_status" "stackit" {
  project_id = module.stackit.project_id
  gateway_id = stackit_vpn_gateway.stackit.gateway_id
}

resource "random_password" "psk" {
  length  = 32
  special = false
}

resource "stackit_vpn_connection" "onprem_to_stackit" {
  project_id   = module.onprem.project_id
  gateway_id   = stackit_vpn_gateway.onprem.gateway_id
  display_name = "to-stackit"

  tunnel1 = {
    remote_address            = data.stackit_vpn_gateway_status.stackit.tunnels[0].public_ip
    pre_shared_key_wo         = random_password.psk.result
    pre_shared_key_wo_version = 1
    bgp                       = { remote_asn = 64513 }
    peering                   = { local_address = "169.254.0.1", remote_address = "169.254.0.2" }
    phase1 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
    phase2 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
  }

  tunnel2 = {
    remote_address            = data.stackit_vpn_gateway_status.stackit.tunnels[1].public_ip
    pre_shared_key_wo         = random_password.psk.result
    pre_shared_key_wo_version = 1
    bgp                       = { remote_asn = 64513 }
    peering                   = { local_address = "169.254.1.1", remote_address = "169.254.1.2" }
    phase1 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
    phase2 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
  }
}

resource "stackit_vpn_connection" "stackit_to_onprem" {
  project_id   = module.stackit.project_id
  gateway_id   = stackit_vpn_gateway.stackit.gateway_id
  display_name = "to-onprem"

  tunnel1 = {
    remote_address            = data.stackit_vpn_gateway_status.onprem.tunnels[0].public_ip
    pre_shared_key_wo         = random_password.psk.result
    pre_shared_key_wo_version = 1
    bgp                       = { remote_asn = 64512 }
    peering                   = { local_address = "169.254.0.2", remote_address = "169.254.0.1" }
    phase1 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
    phase2 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
  }

  tunnel2 = {
    remote_address            = data.stackit_vpn_gateway_status.onprem.tunnels[1].public_ip
    pre_shared_key_wo         = random_password.psk.result
    pre_shared_key_wo_version = 1
    bgp                       = { remote_asn = 64512 }
    peering                   = { local_address = "169.254.1.2", remote_address = "169.254.1.1" }
    phase1 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
    phase2 = {
      dh_groups             = ["modp2048"]
      encryption_algorithms = ["aes256gcm16"]
      integrity_algorithms  = ["sha2_256"]
    }
  }
}
