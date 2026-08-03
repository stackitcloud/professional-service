# 003-spoke-project

A spoke project: two Windows Server instances attached to the shared network area created by
[`001-hub-project`](../001-hub-project). All outbound traffic routes through the OPNsense hub.

See the [top-level README](../README.md) for the full architecture and repository overview.

---

## What this deploys

- A STACKIT project attached to the hub's shared network area (SNA)
- A dedicated subnet (default `10.28.2.0/28`) with a routing table pointing the default route at
  the OPNsense LAN IP
- Two example Windows Server instances via the generic [`modules/server`](../modules/server) module

| Server             | Machine Type | Purpose                      |
| ------------------ | ------------ | ----------------------------- |
| `windows-server-a` | `m2i.8`      | Standard Windows workload      |
| `windows-server-b` | `n2.14d.g1`  | GPU-enabled Windows workload   |

---

## Files

| File                | Purpose                                                             |
| -------------------- | ---------------------------------------------------------------------- |
| `000-backend.tf`     | S3-compatible remote state backend                                     |
| `000-variables.tf`   | Input variables (org/folder IDs, `spoke_subnet`, `hub_firewall_lan_ip`) |
| `010-provider.tf`    | STACKIT provider configuration                                         |
| `020-projects.tf`    | STACKIT project attached to the hub's network area                     |
| `030-network.tf`     | Spoke subnet + routing table (default route → OPNsense LAN)            |
| `040-servers.tf`     | `windows-server-a` and `windows-server-b`, via `modules/server`         |
| `050-outputs.tf`     | `spoke_project_id`, `windows_server_a_ip`, `windows_server_b_ip`       |
| `backend.conf.example` | Backend credential template — copy to `backend.conf`                 |

---

## Required variables

| Variable                   | Description                                                              |
| ---------------------------- | ---------------------------------------------------------------------------- |
| `stackit_organization_id`    | STACKIT organization UUID                                                     |
| `stackit_folder_id`          | Folder that will contain this project                                        |
| `stackit_network_area_id`    | Output of `001-hub-project` (`terraform output network_area_id`)             |
| `org_admin`                  | Email of the STACKIT user set as project owner                               |
| `spoke_subnet`                | This spoke's network prefix (default `10.28.2.0/28`)                        |
| `hub_firewall_lan_ip`        | OPNsense LAN IP, output of `001-hub-project` (default `10.28.0.20`)          |

See `000-variables.tf` for the full list, defaults, and descriptions.

## Outputs

| Output                | Description                                 |
| ----------------------- | ------------------------------------------------ |
| `spoke_project_id`      | STACKIT project ID of this spoke                  |
| `windows_server_a_ip`   | Primary IP of `windows-server-a`                  |
| `windows_server_b_ip`   | Primary IP of `windows-server-b`                  |

---

## Deploy

Requires `001-hub-project` to be deployed first.

```sh
cp backend.conf.example backend.conf   # fill in bucket + S3 credentials
cp ../terraform.tfvars.example terraform.tfvars  # fill in variable values
# place your service account key at keys/service-account.json

# from the hub project:
#   terraform output network_area_id  → stackit_network_area_id
#   terraform output firewall_lan_ip  → hub_firewall_lan_ip

terraform init -backend-config=backend.conf
terraform apply
```
