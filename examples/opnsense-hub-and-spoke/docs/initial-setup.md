# Initial OPNsense Setup

Terraform provisions the OPNsense VM, its three network interfaces, and the public IPs — but OPNsense itself
ships unconfigured, with no automated first-boot hook (see [`docs/mgmt-standardization.md`](mgmt-standardization.md)
for the constraints behind this). This guide covers the steps to take right after `terraform apply` succeeds
in `001-hub-project`.

Only **WAN** and **LAN** are assigned automatically. The MGMT NIC (`nic_mgmt`, `10.28.0.32/28`) exists at the
STACKIT network level from Terraform, but OPNsense itself doesn't know about it yet — assigning it is part of
this setup (step 2).

## 1. First web UI login — via WAN

Log in over the **WAN** public IP for this initial setup (MGMT doesn't exist as an OPNsense interface yet, so
it can't be used for anything until after step 2):

```sh
terraform -chdir=001-hub-project output wan_public_ip
```

Browse to `https://<wan_public_ip>/`. The self-signed certificate will trigger a browser warning — accept it
for initial setup (replace it with a real certificate before any production use). Log in with:

```
Username: root
Password: STACKIT123!
```

**Change the root password immediately** in this session (**System → Access → Users → root**) — this is a
published default and WAN is internet-facing.

## 2. Assign the MGMT interface (OPT1)

OPNsense has no API endpoint for assigning a physical NIC to an interface role — this is a one-time GUI step.

1. **Interfaces → Assignments**. The third NIC shows up as an unassigned device (`vtnet2`) under
   "Assign a new interface" — select it, click **Add**. It's created as `OPT1`.
2. Open the new **[OPT1]** interface page: **Enable** it, set **IPv4 Configuration Type** to **DHCP**, Save,
   Apply. STACKIT's network fabric hands out the exact IP/gateway Terraform reserved for this NIC
   (`10.28.0.36/28`, gateway `10.28.0.33`) via DHCP — you don't need to hardcode them.

This example refers to the interface by its default identifier, `OPT1` — renaming it is optional and purely
cosmetic.

## 3. About long-term MGMT access

OPT1 (MGMT) doesn't have a public IP path fully wired up by default the way WAN does — reaching its web UI
from outside the network area over its own public IP hits the same asymmetric-routing problem covered in
[`docs/webui-access.md`](webui-access.md). Read that next; it covers the fix and how to switch primary admin
access from WAN to MGMT afterward.

## Next steps

- [`docs/webui-access.md`](webui-access.md) — the MGMT routing gotcha and its fix, and switching admin access
  from WAN to MGMT.
- [`docs/mgmt-standardization.md`](mgmt-standardization.md) — background on why this setup can't be further
  automated.
