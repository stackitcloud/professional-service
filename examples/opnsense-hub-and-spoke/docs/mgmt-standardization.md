# Management Setup: Automation Evaluation

Issue #51 asked whether the MGMT interface setup and public web UI access fix
([`docs/webui-access.md`](webui-access.md)) can be standardized and deployed reproducibly through Ansible as
part of the OPNsense IaC setup. This documents that evaluation and its conclusion.

## What can't be automated, regardless of tooling

- **Interface assignment.** The OPNsense API has no endpoint for assigning a physical NIC to an interface
  role. Terraform reserving `10.28.2.100` for the MGMT NIC at the network level does not make OPNsense aware
  of that interface — `vtnet2` stays unassigned until it's added via **Interfaces → Assignments**. No
  Ansible module, collection, or first-boot mechanism changes this — it's an OPNsense API limitation, not a
  tooling gap.
- **No first-boot hook.** This OPNsense image has no cloud-init or other handler for `user_data` passed via
  `stackit_server` (the mechanism that does work for this repo's Linux/Windows spoke servers), so there is
  no way to inject configuration at VM creation time either.

Given this, MGMT interface assignment (`docs/initial-setup.md`) stays a one-time manual GUI step per
firewall instance — no automation approach removes it.

## What could be automated, and why it isn't in this example

Once OPT1/MGMT is assigned, the specific firewall rules from the [validated fix](webui-access.md) (an
interface-scoped allow rule with an explicit reply-to gateway) are configurable through OPNsense's REST API,
and community Ansible collections exist for it. For this example, that layer of automation was evaluated and
intentionally left out: the amount of configuration involved (two firewall rules) doesn't justify the added
tooling dependency (API credential management, a collection to install and keep compatible with the OPNsense
version in use) for a reference implementation. Configure the rules manually per
[`docs/webui-access.md`](webui-access.md) instead.

If your deployment grows beyond a couple of rules — more firewall policy, NAT, DHCP, VPN — revisit
Ansible- or Terraform-based configuration against the OPNsense REST API at that point; see
[`docs/automation-options.md`](automation-options.md) for what's available. The interface-assignment step
above remains manual either way.

## What's already standardized by Terraform

- **Fixed IP:** the MGMT NIC gets `10.28.2.100` explicitly (`stackit_network_interface.nic_mgmt` in
  [`030-network.tf`](../001-hub-project/030-network.tf)).
- **Fixed device order:** MGMT is always the third NIC attached
  (`stackit_server_network_interface_attach.attach_mgmt` in
  [`040-hub-fw-opnsense.tf`](../001-hub-project/040-hub-fw-opnsense.tf) depends on `attach_lan`, which
  depends on the boot NIC), so it's consistently `vtnet2`.

This is what makes the DHCP-based assignment in `docs/initial-setup.md` reliable across redeploys: the
device to assign is always the same one, and it always receives the same reserved IP once assigned.

## Consequence for `terraform apply` reruns

Because the OPT1 assignment lives in OPNsense's own configuration (not in Terraform state), destroying and
recreating the `opnsense` server requires reassigning it — this is easy to overlook when rebuilding the
firewall.
