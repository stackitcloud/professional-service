# Hub-and-Spoke VPN on STACKIT — OPNsense Reference Implementation

A reference implementation of a **hub-and-spoke network topology** on [STACKIT](https://www.stackit.de/), provisioned with Terraform.

The hub deploys an **OPNsense firewall** as the central routing and security component. All spoke traffic is forwarded through OPNsense for routing, NAT, and policy enforcement. Each project is a self-contained Terraform stack with independent state.

---

## Architecture

```
                              Internet
                                  |
  +-------------------------------------------------------------------+
  |                      STACKIT Network Area                         |
  |                                                                   |
  |   +-------------------------------------------------------------+ |
  |   |                     001-hub-project                         | |
  |   |                                                             | |
  |   |   OPNsense Firewall                                         | |
  |   |   +------------------+------------------+                   | |
  |   |   | Interface        | IP               |                   | |
  |   |   +------------------+------------------+                   | |
  |   |   | WAN              | 10.28.0.4        |                   | |
  |   |   | LAN              | 10.28.0.20  <--  |                   | |
  |   |   | MGMT             | 10.28.0.36       |                   | |
  |   |   +------------------+------------------+                   | |
  |   |                                                             | |
  |   |   default route next-hop: 10.28.0.20                        | |
  |   +---------------------------+---------------------------------+ |
  |                               |                                   |
  |              +----------------+----------------+                  |
  |              |                                 |                  |
  |   +----------+----------+         +------------+---------+        |
  |   |   002-spoke-project |         |  003-spoke-project   |        |
  |   |   10.28.1.0/28      |         |  10.28.2.0/28        |        |
  |   +---------------------+         +----------------------+        |
  +-------------------------------------------------------------------+
```

**Traffic flow:** All spoke traffic (including internet-bound) is forwarded to the OPNsense LAN NIC (`10.28.0.20`) via a routing table route attached to each spoke network. OPNsense handles routing, NAT, and firewall policy centrally.

---

## Repository Structure

```
hub-and-spoke-vpn/
├── 001-hub-project/           # Hub: OPNsense firewall, network area, routing tables
│   ├── 000-backend.tf         # S3 remote state backend
│   ├── 000-variables.tf       # Input variables (opnsense_machine_type, mgmt_ip_range)
│   ├── 010-provider.tf        # STACKIT provider
│   ├── 020-projects.tf        # STACKIT project + shared network area (SNA)
│   ├── 030-network.tf         # Subnets, routing tables, NICs, security groups
│   ├── 040-hub-fw-opnsense.tf # OPNsense image, volume, server, public IPs
│   ├── 050-outputs.tf         # network_area_id, firewall_lan_ip, public IPs (needed by spokes)
│   └── backend.conf.example   # Backend credential template
│
├── 002-spoke-project/        # Spoke A: example Linux servers (RHEL 9)
│   ├── 000-backend.tf
│   ├── 000-variables.tf
│   ├── 010-provider.tf
│   ├── 020-projects.tf
│   ├── 030-network.tf        # Spoke subnet + routing table (default → OPNsense LAN)
│   ├── 040-servers.tf        # Two Linux server examples (different machine types)
│   ├── 050-outputs.tf
│   └── backend.conf.example
│
├── 003-spoke-project/        # Spoke B: example Windows Server instances
│   ├── 000-backend.tf
│   ├── 000-variables.tf
│   ├── 010-provider.tf
│   ├── 020-projects.tf
│   ├── 030-network.tf
│   ├── 040-servers.tf        # Two Windows server examples (standard + GPU-enabled)
│   ├── 050-outputs.tf
│   └── backend.conf.example
│
├── modules/
│   └── server/               # Generic compute module (volume + NIC + server)
│       ├── main.tf
│       ├── variables.tf      # Pass image_id to choose OS; see variable description
│       └── outputs.tf
│
├── cloud-init/
│   ├── user-init-linux.yml   # Cloud-init for Linux instances
│   └── user-init-windows.yml # Cloud-init for Windows instances
│
├── terraform.tfvars.example  # Variable template for all projects
└── Architecture/
    └── hub-and-spoke.drawio  # Architecture diagram (draw.io)
```

**File numbering convention:** Files within each project are numbered to make the dependency and deployment order explicit:

- `000` — backend and variables (no dependencies)
- `010` — provider configuration
- `020` — STACKIT project and network area
- `030` — networking (routing tables, subnets, interfaces)
- `040` — compute (VMs)
- `050` — outputs

---

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
- A STACKIT account with an organization, a folder, and sufficient IAM permissions
- A STACKIT service account with a JSON key file
- A STACKIT Object Storage bucket for Terraform remote state
- S3-compatible access credentials for that bucket

---

## Setup

### 1. Service account key

Download your STACKIT service account key from the portal and place it in the `keys/` directory of each project you want to deploy:

```
001-hub-project/keys/service-account.json
002-spoke-project/keys/service-account.json
003-spoke-project/keys/service-account.json
```

The `keys/` directory is gitignored. Never commit key files.

### 2. Backend credentials

Copy `backend.conf.example` to `backend.conf` in each project directory:

```sh
cp 001-hub-project/backend.conf.example 001-hub-project/backend.conf
# edit the file and fill in your bucket name and credentials
```

`backend.conf` is gitignored. Initialize Terraform with:

```sh
terraform init -backend-config=backend.conf
```

### 3. Variable values

Copy `terraform.tfvars.example` into each project directory and fill in your values:

```sh
cp terraform.tfvars.example 001-hub-project/terraform.tfvars
cp terraform.tfvars.example 002-spoke-project/terraform.tfvars
cp terraform.tfvars.example 003-spoke-project/terraform.tfvars
# edit each file
```

The minimum required values are documented in each `000-variables.tf`.

---

## Deployment Order

Deploy in numbered order. The hub creates the shared network area that spokes depend on.

```sh
# Step 1 — Deploy the hub (creates the network area and the OPNsense firewall)
cd 001-hub-project
terraform init -backend-config=backend.conf
terraform apply

# Step 2 — Copy outputs into spoke terraform.tfvars
terraform output network_area_id  # → set as stackit_network_area_id in spokes
terraform output firewall_lan_ip  # → set as hub_firewall_lan_ip in spokes (default: 10.28.0.20)

# Step 3 — Deploy spokes (independently, in any order)
cd ../002-spoke-project
terraform init -backend-config=backend.conf
terraform apply

cd ../003-spoke-project
terraform init -backend-config=backend.conf
terraform apply
```

---

## Hub Firewall (OPNsense)

OPNsense is provisioned from a qcow2 image with three network interfaces:

| Interface | Subnet          | IP           | Purpose                    |
| --------- | --------------- | ------------ | -------------------------- |
| WAN       | `10.28.0.0/28`  | `10.28.0.4`  | Internet uplink            |
| LAN       | `10.28.0.16/28` | `10.28.0.20` | Default gateway for spokes |
| MGMT      | `10.28.0.32/28` | `10.28.0.36` | Web UI / SSH access        |

**Default credentials:**

| Field    | Value         |
| -------- | ------------- |
| Username | `root`        |
| Password | `STACKIT123!` |

The WAN interface boots first (`vtnet0`); LAN and MGMT are attached sequentially and appear as `vtnet1` and `vtnet2`. The MGMT interface is protected by a security group that restricts SSH, HTTP, and HTTPS access to the CIDR set in `mgmt_ip_range`.

**OPNsense image:** The image is downloaded automatically during `terraform apply` via a `null_resource` provisioner. The qcow2 image is fetched from the STACKIT Object Storage endpoint and uploaded to STACKIT as a custom image.

---

## Spoke Projects

### 002-spoke-project — Linux servers (RHEL 9)

Two servers showing different machine type profiles:

| Server     | Machine Type | Purpose                  |
| ---------- | ------------ | ------------------------ |
| `server-a` | `c2i.2`      | General-purpose compute  |
| `server-b` | `m1a.8d`     | Memory-optimized compute |

### 003-spoke-project — Windows Server instances

Two servers showing Windows with different compute profiles:

| Server             | Machine Type | Purpose                      |
| ------------------ | ------------ | ---------------------------- |
| `windows-server-a` | `m2i.8`      | Standard Windows workload    |
| `windows-server-b` | `n2.14d.g1`  | GPU-enabled Windows workload |

---

## Server Module (`modules/server`)

A single generic module used by all spokes. Select the OS by passing the appropriate `image_id`. Supported operating systems include:

- RHEL 9 _(default)_
- Windows Server 2022
- Debian 12

**Image UUIDs:** Image IDs change between releases and vary by region. Retrieve the current image UUIDs from STACKIT before deploying. Set the appropriate `image_id` value in your Terraform configuration.

---

## What You Must Adapt Before Use

| Value                     | Where                                   | Description                                             |
| ------------------------- | --------------------------------------- | ------------------------------------------------------- |
| `stackit_organization_id` | all `000-variables.tf` / tfvars         | Your STACKIT org UUID                                   |
| `stackit_folder_id`       | all `000-variables.tf` / tfvars         | Folder that contains the projects                       |
| `stackit_network_area_id` | spoke `000-variables.tf` / tfvars       | Output of `001-hub-project`                             |
| `org_admin`               | all `000-variables.tf` / tfvars         | Project owner email                                     |
| `mgmt_ip_range`           | `001-hub-project` tfvars                | CIDR allowed to reach the firewall management interface |
| Backend credentials       | `backend.conf` per project              | Object Storage bucket + S3 keys                         |
| Service account key       | `keys/service-account.json` per project | Downloaded from STACKIT portal                          |
| Cloud-init password       | `cloud-init/*.yml`                      | Replace placeholder hash with a real one                |

---

## References

- [STACKIT Terraform Provider](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs)
- [STACKIT Documentation](https://docs.stackit.cloud/)
- [OPNsense Documentation](https://docs.opnsense.org/)
