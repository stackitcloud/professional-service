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

module "vpn_sna_01" {
  source                    = "../../modules/stackit-sna-with-debug-machine"
  machine_availability_zone = "eu01-1"
  machine_ipv4_prefix       = "10.10.10.0/24"
  machine_network_name      = "vpn-sna-01"
  sna_name                  = "vpn-sna-01"
  machine_name              = "vpn-sna-01"
  stackit_admin_email       = var.stackit_admin_email
  stackit_org_id            = var.stackit_org_id
  stackit_project_name      = "vpn-sna-01"
  sna_network_range_prefix = [
    "10.10.0.0/16"
  ]
}

module "vpn_sna_02" {
  source                    = "../../modules/stackit-sna-with-debug-machine"
  machine_availability_zone = "eu01-2"
  machine_ipv4_prefix       = "10.11.11.0/24"
  machine_network_name      = "vpn-sna-02"
  machine_name              = "vpn-sna-02"
  sna_name                  = "vpn-sna-02"
  stackit_admin_email       = var.stackit_admin_email
  stackit_org_id            = var.stackit_org_id
  stackit_project_name      = "vpn-sna-02"
  sna_network_range_prefix = [
    "10.11.0.0/16"
  ]
}

# Gateway 1 (vpn-sna-01)
resource "stackit_vpn_gateway" "vpn_01_gateway" {
  project_id   = module.vpn_sna_01.project_id
  display_name = "vpn01"
  plan_id      = "p500"
  routing_type = "BGP_ROUTE_BASED"

  availability_zones = {
    tunnel1 = "eu01-1"
    tunnel2 = "eu01-2"
  }

  bgp = {
    local_asn                  = 64512
    override_advertised_routes = ["10.10.0.0/16"]
  }
}

data "stackit_vpn_gateway_status" "vpn_01_gateway_status" {
  project_id = module.vpn_sna_01.project_id
  gateway_id = stackit_vpn_gateway.vpn_01_gateway.gateway_id
}

# Gateway 2 (vpn-sna-02)
resource "stackit_vpn_gateway" "vpn_02_gateway" {
  project_id   = module.vpn_sna_02.project_id
  display_name = "vpn02"
  plan_id      = "p500"
  routing_type = "BGP_ROUTE_BASED"

  availability_zones = {
    tunnel1 = "eu01-1"
    tunnel2 = "eu01-2"
  }

  bgp = {
    local_asn                  = 64513
    override_advertised_routes = ["10.11.0.0/16"]
  }
}

data "stackit_vpn_gateway_status" "vpn_02_gateway_status" {
  project_id = module.vpn_sna_02.project_id
  gateway_id = stackit_vpn_gateway.vpn_02_gateway.gateway_id
}

# Shared VPN Credentials
resource "random_password" "vpn_psk" {
  length  = 32
  special = false
}

# Connection from Gateway 1 to Gateway 2
resource "stackit_vpn_connection" "vpn_01_connection" {
  project_id   = module.vpn_sna_01.project_id
  gateway_id   = stackit_vpn_gateway.vpn_01_gateway.gateway_id
  display_name = "conn-to-vpn02"

  tunnel1 = {
    remote_address            = data.stackit_vpn_gateway_status.vpn_02_gateway_status.tunnels[0].public_ip
    pre_shared_key_wo         = random_password.vpn_psk.result
    pre_shared_key_wo_version = 1

    bgp = {
      remote_asn = 64513
    }
    peering = {
      local_address  = "169.254.0.1"
      remote_address = "169.254.0.2"
    }
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
    remote_address            = data.stackit_vpn_gateway_status.vpn_02_gateway_status.tunnels[1].public_ip
    pre_shared_key_wo         = random_password.vpn_psk.result
    pre_shared_key_wo_version = 1

    bgp = {
      remote_asn = 64513
    }
    peering = {
      local_address  = "169.254.1.1"
      remote_address = "169.254.1.2"
    }
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

# Connection from Gateway 2 to Gateway 1
resource "stackit_vpn_connection" "vpn_02_connection" {
  project_id   = module.vpn_sna_02.project_id
  gateway_id   = stackit_vpn_gateway.vpn_02_gateway.gateway_id
  display_name = "conn-to-vpn01"

  tunnel1 = {
    remote_address            = data.stackit_vpn_gateway_status.vpn_01_gateway_status.tunnels[0].public_ip
    pre_shared_key_wo         = random_password.vpn_psk.result
    pre_shared_key_wo_version = 1

    bgp = {
      remote_asn = 64512
    }
    peering = {
      local_address  = "169.254.0.2"
      remote_address = "169.254.0.1"
    }
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
    remote_address            = data.stackit_vpn_gateway_status.vpn_01_gateway_status.tunnels[1].public_ip
    pre_shared_key_wo         = random_password.vpn_psk.result
    pre_shared_key_wo_version = 1

    bgp = {
      remote_asn = 64512
    }
    peering = {
      local_address  = "169.254.1.2"
      remote_address = "169.254.1.1"
    }
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

output "vpn01_public_ip" {
  value = module.vpn_sna_01.machine_public_ip
}

output "vpn01_private_ip" {
  value = module.vpn_sna_01.machine_private_ipv4
}

output "vpn02_public_ip" {
  value = module.vpn_sna_02.machine_public_ip
}

output "vpn02_private_ip" {
  value = module.vpn_sna_02.machine_private_ipv4
}
