# Known Limitations

## No OPNsense High Availability (CARP)

This example deploys a **single** OPNsense instance. There is no second firewall node, no dedicated CARP
sync interface, and no virtual IP (VIP) failover configured — none of the Terraform in `001-hub-project`
provisions the second node or sync network a CARP pair requires.

Practical implications:

- A reboot, host maintenance event, or OPNsense failure is a routing outage for every spoke until the
  instance recovers — there's no automatic failover.
- Do not treat `10.28.1.100` (the LAN gateway spokes route through) as a highly-available address; it's a
  single VM's NIC.

### If you need resilience: load-balancer sandwich, not CARP

STACKIT does not currently support the elastic/floating IP handoff that OPNsense CARP relies on between two
VMs in the same network. The supported pattern for firewall resilience on STACKIT is a **load-balancer
sandwich**: an STACKIT L3 Load Balancer in front of a pool of OPNsense instances (and, if the workload
needs it, a second one behind, load-balancing egress across the pool). The load balancer's health checks
replace CARP's failover role — an unhealthy OPNsense instance is taken out of rotation instead of a VIP
migrating to a standby.

This is **out of scope for this example**: it changes the network topology (multiple OPNsense instances
behind a shared frontend, health-check endpoints, session-affinity considerations for stateful firewall
rules) enough that it warrants its own reference implementation rather than a variant of this one. If you
need this, treat `001-hub-project` as a starting point for the single-instance leg of that design, not as a
drop-in.

## MGMT interface setup is not automated

See [`docs/mgmt-standardization.md`](mgmt-standardization.md) — this OPNsense image has no cloud-init or
other first-boot hook, and the OPNsense API has no endpoint for assigning a physical interface. Assigning
MGMT (OPT1) is a manual, one-time GUI step per firewall instance — see
[`docs/initial-setup.md`](initial-setup.md).
