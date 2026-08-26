<!-- tags: vpn, networking, ipsec, site-to-site, gcp, cross-cloud, ha, bgp -->

# STACKIT-to-GCP HA VPN Gateway

This example demonstrates how to establish a secure, Highly Available (HA) IPsec VPN connection between a STACKIT Network Area (SNA) and Google Cloud Platform (GCP).

The connection uses **BGP (Border Gateway Protocol)** via a GCP Cloud Router to automatically exchange and propagate routes dynamically between the two cloud environments.

## Architecture

This Terraform configuration provisions the following resources:

- **STACKIT:** An SNA, a debug machine, and an HA VPN Gateway (`ASN 64512`).
- **GCP:** A VPC, a Subnet (`europe-west4`), an HA VPN Gateway, a Cloud Router (`ASN 64513`), and a private Debian test VM.
- **VPN Connection:** Two redundant IPsec tunnels using dynamically generated PSKs and Link-Local BGP peering (`169.254.x.x/30`).
- **Security:** GCP Firewall rules configured to allow Identity-Aware Proxy (IAP) SSH access and inbound ICMP/SSH traffic from the STACKIT network.

## Prerequisites

- Configured STACKIT and GCP provider credentials.
- Google Cloud SDK (`gcloud` CLI) installed and authenticated for IAP testing.

## How to Test the Connection

Once `terraform apply` is complete, check the generated outputs. You can verify the bi-directional tunnel is fully operational by following these steps:

### 1. Test from GCP to STACKIT

Connect to the private GCP VM using Google's Identity-Aware Proxy (IAP) and ping the STACKIT debug machine.

```bash
# 1. SSH into the GCP test VM (copy this from the `gcp_iap_command` output)
gcloud compute ssh gcp-vpn-test-vm --zone=europe-west4-a --tunnel-through-iap

# 2. Ping the STACKIT private IP (copy from the `vpn01_private_ip` output)
ping <vpn01_private_ip>
```

### 2. Test from STACKIT to GCP

Connect to the STACKIT debug machine using its public IP, then ping the private GCP VM across the VPN tunnel.

```Bash
# 1. SSH into the STACKIT debug machine
ssh debug@<vpn01_public_ip>
# password debug123

# 2. Ping the GCP private IP (copy from the `gcp_test_vm_private_ip` output)
ping <gcp_test_vm_private_ip>
```
