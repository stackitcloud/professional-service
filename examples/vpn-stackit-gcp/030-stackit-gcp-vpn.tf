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

# STACKIT Side (vpn-sna-01)
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

resource "random_password" "vpn_psk" {
  length  = 32
  special = false
}

# GCP VPC and Subnet
resource "google_compute_network" "gcp_vpc" {
  name                    = "gcp-vpn-network"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "gcp_subnet" {
  name          = "gcp-vpn-subnet"
  ip_cidr_range = "10.11.0.0/16"
  region        = "europe-west4"
  network       = google_compute_network.gcp_vpc.id
}

# GCP HA VPN Gateway
resource "google_compute_ha_vpn_gateway" "gcp_gateway" {
  name    = "gcp-ha-vpn"
  network = google_compute_network.gcp_vpc.id
  region  = "europe-west4"
}

# GCP Cloud Router (for BGP)
resource "google_compute_router" "gcp_router" {
  name    = "gcp-router"
  network = google_compute_network.gcp_vpc.name
  region  = "europe-west4"
  bgp {
    asn = 64513 # GCP's local ASN
  }
}

# GCP External VPN Gateway (Represents STACKIT in GCP)
resource "google_compute_external_vpn_gateway" "stackit_gateway" {
  name            = "stackit-external-gw"
  redundancy_type = "TWO_IPS_REDUNDANCY"
  description     = "STACKIT VPN Gateway"

  # Fetching the public IPs from STACKIT
  interface {
    id         = 0
    ip_address = data.stackit_vpn_gateway_status.vpn_01_gateway_status.tunnels[0].public_ip
  }
  interface {
    id         = 1
    ip_address = data.stackit_vpn_gateway_status.vpn_01_gateway_status.tunnels[1].public_ip
  }
}

# GCP VPN Tunnels
resource "google_compute_vpn_tunnel" "gcp_tunnel1" {
  name                            = "gcp-tunnel-1"
  region                          = "europe-west4"
  vpn_gateway                     = google_compute_ha_vpn_gateway.gcp_gateway.id
  peer_external_gateway           = google_compute_external_vpn_gateway.stackit_gateway.id
  peer_external_gateway_interface = 0
  shared_secret                   = random_password.vpn_psk.result
  router                          = google_compute_router.gcp_router.id
  vpn_gateway_interface           = 0
}

resource "google_compute_vpn_tunnel" "gcp_tunnel2" {
  name                            = "gcp-tunnel-2"
  region                          = "europe-west4"
  vpn_gateway                     = google_compute_ha_vpn_gateway.gcp_gateway.id
  peer_external_gateway           = google_compute_external_vpn_gateway.stackit_gateway.id
  peer_external_gateway_interface = 1
  shared_secret                   = random_password.vpn_psk.result
  router                          = google_compute_router.gcp_router.id
  vpn_gateway_interface           = 1
}

# GCP Cloud Router Interfaces & BGP Peers
resource "google_compute_router_interface" "gcp_router_interface1" {
  name       = "gcp-interface-1"
  router     = google_compute_router.gcp_router.name
  region     = "europe-west4"
  ip_range   = "169.254.0.2/30" # GCP's local BGP IP
  vpn_tunnel = google_compute_vpn_tunnel.gcp_tunnel1.name
}

resource "google_compute_router_peer" "gcp_router_peer1" {
  name                      = "gcp-peer-1"
  router                    = google_compute_router.gcp_router.name
  region                    = "europe-west4"
  peer_ip_address           = "169.254.0.1" # STACKIT's local BGP IP
  peer_asn                  = 64512         # STACKIT's ASN
  advertised_route_priority = 100
  interface                 = google_compute_router_interface.gcp_router_interface1.name
}

resource "google_compute_router_interface" "gcp_router_interface2" {
  name       = "gcp-interface-2"
  router     = google_compute_router.gcp_router.name
  region     = "europe-west4"
  ip_range   = "169.254.1.2/30"
  vpn_tunnel = google_compute_vpn_tunnel.gcp_tunnel2.name
}

resource "google_compute_router_peer" "gcp_router_peer2" {
  name                      = "gcp-peer-2"
  router                    = google_compute_router.gcp_router.name
  region                    = "europe-west4"
  peer_ip_address           = "169.254.1.1"
  peer_asn                  = 64512
  advertised_route_priority = 100
  interface                 = google_compute_router_interface.gcp_router_interface2.name
}

# Connection from STACKIT to GCP
resource "stackit_vpn_connection" "vpn_01_connection" {
  project_id   = module.vpn_sna_01.project_id
  gateway_id   = stackit_vpn_gateway.vpn_01_gateway.gateway_id
  display_name = "conn-to-gcp"

  tunnel1 = {
    remote_address            = google_compute_ha_vpn_gateway.gcp_gateway.vpn_interfaces[0].ip_address
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
    remote_address            = google_compute_ha_vpn_gateway.gcp_gateway.vpn_interfaces[1].ip_address
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

# GCP Test VM & Firewall Rules
# Firewall: Allow Identity-Aware Proxy (IAP) to SSH into the VM
resource "google_compute_firewall" "allow_iap_ssh" {
  name    = "allow-iap-ssh"
  network = google_compute_network.gcp_vpc.name

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["test-vm"]
}

# Firewall: Allow STACKIT to ping/SSH into the GCP VM over the VPN
resource "google_compute_firewall" "allow_stackit_vpn_traffic" {
  name    = "allow-stackit-vpn-traffic"
  network = google_compute_network.gcp_vpc.name

  allow {
    protocol = "icmp"
  }
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  # Your STACKIT SNA range
  source_ranges = ["10.10.0.0/16"]
  target_tags   = ["test-vm"]
}

# GCP Virtual Machine
resource "google_compute_instance" "gcp_test_vm" {
  name         = "gcp-vpn-test-vm"
  machine_type = "e2-micro"
  zone         = "europe-west4-a"

  tags = ["test-vm"]

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.gcp_subnet.id
    # Omitting the 'access_config' block ensures this VM gets NO public IP address.
  }
}

# Outputs
output "vpn01_public_ip" {
  value = module.vpn_sna_01.machine_public_ip
}

output "vpn01_private_ip" {
  value = module.vpn_sna_01.machine_private_ipv4
}

output "gcp_test_vm_private_ip" {
  value = google_compute_instance.gcp_test_vm.network_interface[0].network_ip
}

output "gcp_iap_command" {
  value = "gcloud compute ssh ${google_compute_instance.gcp_test_vm.name} --tunnel-through-iap"
}
