# Automating OPNsense Configuration: Options

This example uses Terraform (the `stackit` provider) to provision the OPNsense VM and its network — but
that only covers STACKIT-side infrastructure. OPNsense's own configuration (firewall rules, NAT, DHCP, VPN,
DNS, etc.) is not automated here beyond the manual setup in [`docs/initial-setup.md`](initial-setup.md) and
[`docs/webui-access.md`](webui-access.md). This is a pointer to the tooling available if you want to add
that layer yourself — not an implementation.

**Neither option below is vendor-maintained.** Both are independent, community-maintained open-source
projects — not published or supported by Deciso (OPNsense), HashiCorp, or Red Hat/Ansible. Their maintenance
status can change (slow down, stop, or get picked up by someone else) independently of OPNsense itself. If
you adopt either, treat that as an ongoing platform-team responsibility, not a one-time choice: periodically
check that the project is still active (recent commits/releases, open security issues triaged) and that it
still tracks the OPNsense version you're running, before pulling in updates or relying on it for anything
security-relevant.

## Terraform: `browningluke/opnsense` provider

A community Terraform provider ([registry](https://registry.terraform.io/providers/browningluke/opnsense/latest),
[source](https://github.com/browningluke/terraform-provider-opnsense)) that manages OPNsense declaratively
through its REST API — a separate provider from `stackit`, added alongside it if you want OPNsense config
tracked in the same Terraform state as the infrastructure.

Covers:

- Firewall rules, aliases, NAT (source and one-to-one), categories
- VLANs and VIPs, interface overview data
- DHCP (Kea v4/v6: subnets, reservations, peers, PD pools)
- DNS (Dnsmasq hosts, Unbound settings)
- VPN (IPsec, OpenVPN, WireGuard)
- Routes and gateways
- Limited plugin support (Quagga BGP, WireGuard)

Authenticates with an OPNsense API key/secret (`OPNSENSE_API_KEY` / `OPNSENSE_API_SECRET` or provider
config). Pre-1.0 at time of writing — actively developed, no stability guarantee on schema between releases.

## Ansible: `oxlorg.opnsense` collection

A community Ansible collection ([docs](https://ansible-opnsense.oxl.app/), formerly published as
`ansibleguy.opnsense`) with a broad module set against the same REST API — an imperative alternative (or
complement) to the Terraform provider, decoupled from Terraform state.

Covers a similar surface plus more: firewall rules/aliases, NAT, gateways/routes, DHCP, Unbound/dnsmasq/BIND
DNS, IPsec/OpenVPN/WireGuard, HAProxy, IDS, Squid, users/groups, cron, syslog, and a generic `raw` module for
any API endpoint the collection doesn't wrap yet. Also authenticates with an API key/secret.

## What neither can do

Both tools talk to OPNsense's REST API, so both share the same platform-level gaps documented in
[`docs/mgmt-standardization.md`](mgmt-standardization.md):

- No way to assign a physical NIC to an interface role (no API endpoint exists for it).
- No way to enable SSH or create the initial API key/secret (both are GUI-only, one-time steps).

Neither is a tooling limitation — installing either the Terraform provider or the Ansible collection doesn't
change this.

## Choosing between them

- **Terraform provider** fits if you want OPNsense configuration versioned and planned alongside the
  STACKIT infrastructure in one `terraform apply` flow.
- **Ansible collection** fits if you prefer imperative, re-runnable playbooks decoupled from Terraform
  state, or need a specific module the Terraform provider doesn't yet cover (its resource set is currently
  narrower than the Ansible collection's).

This example doesn't use either — see [`docs/mgmt-standardization.md`](mgmt-standardization.md) for why
Ansible specifically was evaluated and left out for the small amount of configuration involved here.
