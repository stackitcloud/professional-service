<!-- tags: iaas, ha, firewall, paloalto, networking, sna, image, custom-image, vip, terraform -->

# Palo Alto VM-Series HA Pair on STACKIT

Terraform code for two **Palo Alto VM-Series firewalls** in an **active/passive High Availability (HA)** setup on STACKIT IaaS. The firewalls run in different availability zones and share a **floating IP (VIP)** with a public IP on the WAN side. The code creates a **STACKIT Network Area (SNA)** with a new project, uploads the PA-VM image and deploys both firewalls with management, HA, WAN and two LAN interfaces.

Terraform only provisions the infrastructure. The HA configuration itself (HA1/HA2 links, VIP, failover, preemption) is done on the firewalls.

---

## Architecture

```mermaid
flowchart TB
    internet((Internet))
    vip["VIP 10.220.131.30<br/>public IP, 1:1 NAT"]

    subgraph pair["HA pair, active/passive"]
        direction LR
        fw1["fw1<br/>eu01-1"] <-- "HA link" --> fw2["fw2<br/>eu01-2"]
    end

    lan["lan_network1 / lan_network2<br/>gateway .1"]

    internet --> vip --> pair
    internet -. "management, public IP per firewall" .-> pair
    pair --> lan
```

The active firewall owns the VIP on `ethernet1/2` and the LAN gateway IPs; the passive firewall takes them over on failover.

### Interface mapping

The order in which the interfaces are attached defines the interface names in PAN-OS. The servers are created with mgmt, HA and WAN; the LAN interfaces are attached afterwards, one after another.

| PAN-OS        | Network        | Prefix            | fw1 / fw2 | Port security                                |
| ------------- | -------------- | ----------------- | --------- | -------------------------------------------- |
| `mgmt`        | `mgmt_network` | `10.220.129.0/24` | .10 / .11 | on, SG `paloalto-mgmt`, public IP            |
| `ethernet1/1` | `ha_network`   | `10.220.254.0/24` | .10 / .11 | off                                          |
| `ethernet1/2` | `wan_network`  | `10.220.131.0/24` | .10 / .11 | on, SG `paloalto-wan`, VIP `.30` (public IP) |
| `ethernet1/3` | `lan_network1` | `10.220.1.0/24`   | .10 / .11 | off, gateway `.1`                            |
| `ethernet1/4` | `lan_network2` | `10.220.2.0/24`   | .10 / .11 | off, gateway `.1`                            |

The host part of the IPs comes from `var.firewalls`, the VIP from `var.vip_host`, the LAN gateway from `var.lan_gateway_host`. The exact addresses are shown in the `interfaces`, `vip` and `lan_gateways` outputs.

### Why a management and an HA network?

- **Management (`mgmt_network`)**: on the VM-Series the first interface is always the management interface. Web UI, SSH, licensing, updates and Panorama run there, separated from the dataplane, so they also work on the passive firewall. HA1 (heartbeat, active/passive election, configuration sync) runs over it as well.
- **HA (`ha_network`)**: carries HA2 on `ethernet1/1` and synchronizes sessions, ARP and forwarding tables and IPsec SAs, so existing connections survive a failover. HA2 needs a dedicated HA interface. Without it, active/passive still works via HA1, but all sessions are dropped on failover.

---

## Components

| File                    | Content                                                                |
| ----------------------- | ---------------------------------------------------------------------- |
| `010-provider.tf`       | Provider (`stackitcloud/stackit`), project module                      |
| `020-variables.tf`      | All variables                                                          |
| `030-network.tf`        | Networks, network interfaces per firewall, VIP port, public IPs        |
| `040-security-group.tf` | Security groups `paloalto-mgmt` and `paloalto-wan`                     |
| `050-image.tf`          | Upload of the PA-VM qcow2 image                                        |
| `060-firewall.tf`       | Firewalls (`for_each` over `var.firewalls`), LAN interface attachments |
| `100-outputs.tf`        | Outputs                                                                |
| `project/`              | Module: network area (SNA) `10.220.0.0/16`, project, wait on destroy   |

### HA failover readiness

The public IP follows the active firewall only if the requirements of the [Firewall HA readiness checklist](https://github.com/stackitcloud/professional-service/blob/main/examples/iaas-ha-vrrp/FIREWALL-HA-READINESS-CHECKLIST.md) are met. Terraform covers the STACKIT side; the rest is PAN-OS configuration:

| Requirement                         | Covered by | How                                                                                                                                                                                                                                                                                                     |
| ----------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Dummy port for the VIP              | Terraform  | `stackit_network_interface.vip` in `wan_network` with a fixed IP, **never attached to a server**. It reserves the address and carries the public IP.                                                                                                                                                    |
| Allowed address pairs on both nodes | Terraform  | Port security is **on** on the WAN ports of both firewalls, with the VIP as `/32` in `allowed_addresses` (a CIDR such as `/24` is not accepted for this). Only then is the VIP port bound to the firewall that sends the GARP. `0.0.0.0/0` is needed in addition, otherwise the failover does not work. |
| No virtual MAC                      | PAN-OS     | Enable **Hypervisor Assigned MAC Address** on both firewalls (Device → Setup → Management → General Settings). Otherwise the active firewall uses an HA virtual MAC, which STACKIT port security treats as MAC spoofing and drops.                                                                      |
| GARP on failover                    | PAN-OS     | With hypervisor-assigned MACs the new active firewall sends gratuitous ARPs for its interface IPs, so the STACKIT router learns the new MAC of the VIP. Verify with the failover test below.                                                                                                            |
| VIP not bound to one node           | PAN-OS     | Active/passive: configure the VIP as the IP of `ethernet1/2`. The configuration is synchronized, only the active firewall uses it. The port IPs `.10` / `.11` are only the STACKIT port addresses.                                                                                                      |
| Session synchronization (optional)  | PAN-OS     | HA1 (control link) over the management interfaces, allowed by the rule `mgmt_ha1`. HA2 (sessions) over `ethernet1/1` in `ha_network`, transport IP or UDP. Without HA2, existing connections are dropped on failover.                                                                                   |

With hypervisor-assigned MACs the MAC of an interface changes on failover. Therefore use static IPs on the dataplane interfaces (no DHCP client) and no EUI-64 IPv6 addresses.

### Security

- `paloalto-mgmt`: ingress TCP 22 and 443 from `var.mgmt_allowed_cidr` (**restrict this to your own IP**), and all traffic between the two management interfaces for HA1.
- `paloalto-wan` on the WAN interfaces: all ingress traffic allowed. Port security is on only because the VIP needs `allowed_addresses`; filtering is the job of the PAN-OS security policies.
- HA and LAN interfaces have port security **off**, because the firewalls send and receive traffic for addresses that are not their own.
- **The public IP of the WAN VIP is open to the internet** by design: the security group lets everything in, every packet reaches the active firewall. Allow the wanted traffic with security policies (and NAT rules) in PAN-OS.
- The management interfaces have a restrictive security group, because the PAN-OS security policies do not protect the management plane.
- New security groups carry the STACKIT default egress rules (all outbound traffic allowed), so the management interface can reach the Palo Alto update and licensing servers.

---

## Getting Started

### Prerequisites

- Terraform ≥ 1.3
- STACKIT service account key with permissions to create network areas and projects in the organization
- Palo Alto VM-Series KVM image (`PA-VM-KVM-<version>.qcow2`) from the Palo Alto support portal, and a license (or a trial) for two VM-Series firewalls

### Deployment

1. Copy the image into this directory (or set `image_file` to its path) and initialize Terraform:

   ```bash
   terraform init
   ```

2. Adjust the defaults in `020-variables.tf`, or copy `terraform.tfvars.example` to `terraform.tfvars`. **Required** (`terraform plan` fails until they are set):

   - `organization_id`: STACKIT organization ID
   - `owner_email`: owner e-mail of the project
   - `service_account_key_path`: service account key JSON, default `~/.stackit/credentials.json`
   - `image_file`: PA-VM qcow2 image, default `./PA-VM-KVM-11.2.5.qcow2`

   Recommended:

   - `mgmt_allowed_cidr`: your own IP, e.g. `1.2.3.4/32`

3. Plan and apply:

   ```bash
   terraform plan
   terraform apply
   ```

   The image upload takes several minutes. The first boot of PAN-OS takes another 10–15 minutes until the web UI answers.

4. Log in to both firewalls via the `management` output (`https://<public-ip>`, user `admin`, change the default password on first login) and configure HA as described in [HA failover readiness](#ha-failover-readiness):

   - Hypervisor Assigned MAC Address on both firewalls, commit
   - HA mode active/passive, HA1 on the management interface (peer address: `mgmt` IP of the other firewall from the `interfaces` output)
   - `ethernet1/1` as HA interface for HA2 with the IP from the `interfaces` output
   - `ethernet1/2` (WAN) with the VIP from the `vip` output, default gateway `10.220.131.1`
   - `ethernet1/3` / `ethernet1/4` (LAN) with the IPs from the `lan_gateways` output
   - priorities and preemption as required
   - security policies and NAT rules for the traffic on the WAN interface; STACKIT does not filter it

5. Test the failover: ping the public IP of the VIP continuously and suspend the active firewall (Device → High Availability → Operational Commands → Suspend local device). Only a few pings may be lost, then the passive firewall answers.

### Variables

| Variable                        | Default                                                  | Description                                                                  |
| ------------------------------- | -------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `organization_id`               | empty (**required**)                                     | STACKIT organization ID                                                      |
| `owner_email`                   | empty (**required**)                                     | Owner e-mail of the project                                                  |
| `project_name`                  | `paloalto-ha`                                            | Name of the project                                                          |
| `service_account_key_path`      | `~/.stackit/credentials.json`                            | Service account key JSON                                                     |
| `default_region`                | `eu01`                                                   | Region                                                                       |
| `image_file`                    | `./PA-VM-KVM-11.2.5.qcow2`                               | PA-VM qcow2 image                                                            |
| `image_name`                    | `PA-VM-KVM-11.2.5`                                       | Name of the image in the project                                             |
| `flavor`                        | `m1.2`                                                   | Machine type of the firewalls                                                |
| `boot_volume_size`              | `100`                                                    | Boot volume size in GB (image minimum 80)                                    |
| `boot_volume_performance_class` | `storage_premium_perf2`                                  | Performance class of the boot volumes                                        |
| `firewalls`                     | `fw1` in `eu01-1` (host 10), `fw2` in `eu01-2` (host 11) | Firewalls: name → availability zone and host part of the IPs                 |
| `vip_host`                      | `30`                                                     | Host part of the VIP in `wan_network`                                        |
| `lan_gateway_host`              | `1`                                                      | Host part of the LAN IP of the active firewall, announced as gateway by DHCP |
| `mgmt_allowed_cidr`             | `0.0.0.0/0`                                              | Source range for the management interfaces, **restrict this**                |

---

## Outputs

- `project_id`: STACKIT project ID
- `vip`: private and public IP of the VIP
- `management`: web UI URL of each firewall
- `interfaces`: private IPs of each firewall, keyed by PAN-OS interface
- `lan_gateways`: LAN IPs of the active firewall (gateway of the LAN networks)

---

## Notes and Limitations

- **HA sync, failover and preemption are not handled by Terraform.** Terraform does not move the VIP; PAN-OS does it with a gratuitous ARP.
- **Interfaces must match on both firewalls.** For HA sync, both firewalls are connected to the same networks in the same order; do not change the order in `060-firewall.tf`.
- **Changing the interfaces recreates the firewall**: `network_interfaces` of a server cannot be updated in place.
- **LAN networks are not routed** (`routed = false`), so the workloads there can only leave through the firewall. DHCP announces the LAN IP of the active firewall (`.1`, `var.lan_gateway_host`) as gateway. In active/passive this IP is synchronized and moves with the active firewall. It must not be the IP of a STACKIT port (such as `.10` / `.11`), because STACKIT binds that IP to the MAC of one firewall.
- **Destroy takes a few minutes longer**: projects are deleted asynchronously, so `terraform destroy` waits `project_deletion_wait` (default `180s`, in `project/020-variables.tf`) before deleting the network area region. If it still fails with "has still active projects", wait a moment and run `terraform destroy` again.
- This setup is intended for **test and POC** environments.

---

## Support

For issues, please create a Ticket

---

**Author**: Michael Sodan
**License**: Apache-2.0
