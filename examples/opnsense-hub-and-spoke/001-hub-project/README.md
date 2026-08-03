# 001-hub-project

The hub of the topology: creates the shared STACKIT network area (SNA) and provisions the
central OPNsense firewall. Spoke projects depend on this project's outputs and must be deployed
after it.

See the [top-level README](../README.md) for the full architecture and repository overview.

---

## What this deploys

- A STACKIT project and a shared network area (SNA) that spoke projects attach to
- Three subnets (WAN, LAN, MGMT) with routing tables, NICs, and security groups
- An OPNsense firewall server, built from a qcow2 image uploaded as a custom STACKIT image
- Public IPs for the WAN and MGMT interfaces

| Interface | Subnet          | IP           | Purpose                    |
| --------- | --------------- | ------------ | -------------------------- |
| WAN       | `10.28.0.0/28`  | `10.28.0.4`  | Internet uplink            |
| LAN       | `10.28.0.16/28` | `10.28.0.20` | Default gateway for spokes |
| MGMT      | `10.28.0.32/28` | `10.28.0.36` | Web UI / SSH access        |

The MGMT interface is restricted by security group to the CIDR set in `mgmt_ip_range`. Default
OPNsense credentials are `root` / `STACKIT123!` — change these on first login.

---

## Files

| File                      | Purpose                                                        |
| ------------------------- | --------------------------------------------------------------- |
| `000-backend.tf`           | S3-compatible remote state backend                              |
| `000-variables.tf`         | Input variables (org/folder IDs, `mgmt_ip_range`, machine type)  |
| `010-provider.tf`          | STACKIT provider configuration                                  |
| `020-projects.tf`          | STACKIT project + shared network area (SNA)                     |
| `030-network.tf`           | Subnets, routing tables, NICs, security groups                  |
| `040-hub-fw-opnsense.tf`   | OPNsense image upload, boot volume, server, public IPs           |
| `050-outputs.tf`           | `network_area_id`, `firewall_lan_ip`, public IPs (used by spokes)|
| `backend.conf.example`     | Backend credential template — copy to `backend.conf`            |

---

## Required variables

| Variable                    | Description                                                   |
| ---------------------------- | --------------------------------------------------------------- |
| `stackit_organization_id`    | STACKIT organization UUID                                        |
| `stackit_folder_id`          | Folder that will contain this project                            |
| `org_admin`                  | Email of the STACKIT user set as project owner                   |
| `mgmt_ip_range`               | CIDR allowed to reach the firewall MGMT interface                 |
| `opnsense_machine_type`       | Machine type for the firewall (default `c2i.2`)                  |

See `000-variables.tf` for the full list, defaults, and descriptions.

## Outputs

| Output             | Used for                                          |
| ------------------- | ---------------------------------------------------- |
| `network_area_id`   | Set as `stackit_network_area_id` in each spoke        |
| `firewall_lan_ip`   | Set as `hub_firewall_lan_ip` in each spoke (default `10.28.0.20`) |
| `wan_public_ip`     | WAN public IP of the firewall                         |
| `mgmt_public_ip`    | MGMT public IP — web UI at `https://<ip>/`            |
| `hub_project_id`    | STACKIT project ID of the hub                         |

---

## Deploy

```sh
cp backend.conf.example backend.conf   # fill in bucket + S3 credentials
cp ../terraform.tfvars.example terraform.tfvars  # fill in variable values
# place your service account key at keys/service-account.json

terraform init -backend-config=backend.conf
terraform apply

terraform output network_area_id
terraform output firewall_lan_ip
```

Copy the two output values into the `terraform.tfvars` of each spoke project before deploying them.

---

## Post-deployment

OPNsense needs manual first-boot configuration — see [`../docs/initial-setup.md`](../docs/initial-setup.md)
and [`../docs/webui-access.md`](../docs/webui-access.md).
