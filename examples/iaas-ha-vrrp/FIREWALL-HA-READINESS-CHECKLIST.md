# Public HA Failover-Readiness Checklist

Use this checklist to verify that a firewall appliance or virtual machine is ready for
active-passive HA failover with a public (floating) IP on STACKIT.
All items are required unless explicitly marked optional.

---

## 1. MAC Handling and Layer-2 Basics (vMAC-less Design)

- [ ] **Native vMAC-less mode**: The appliance must be able to operate entirely without virtual MAC
      addresses. Standard VRRP assigns a dedicated virtual MAC (`00:00:5e:00:01:<vrid>`) which
      OVN treats as MAC spoofing and silently drops.

- [ ] **Use the hypervisor-assigned MAC**: During failover the newly active node must send all
      traffic for the virtual IP (VIP) using its real vNIC MAC — the MAC assigned by the STACKIT
      hypervisor — not a protocol-generated virtual MAC.

---

## 2. Routing Prerequisites (Switching the Public IP / NAT)

- [ ] **Aggressive GARP for all addresses**: On a state transition (BACKUP → MASTER) the appliance
      must automatically send Gratuitous ARP (GARP) packets for _every_ configured VIP and IP alias,
      not only for its primary interface IP. A GARP only for the management IP is insufficient.

- [ ] **Fast convergence**: GARP packets must be sent immediately after the state change and
      ideally repeated several times to ensure the STACKIT upstream router (NAT gateway) reliably
      receives the Layer-2 update and refreshes its ARP cache before the first inbound packet arrives.

---

## 3. OS and Architecture Prerequisites (Appliance Capabilities)

- [ ] **Independent VIPs**: The appliance OS must allow configuring IP addresses that are not
      permanently bound to a single physical interface or automatically synchronized between cluster
      nodes. The VIP must be addable and removable on the active node at runtime.

- [ ] _(Optional)_ **Seamless session synchronization**: For zero-drop failover the two nodes
      should synchronize their connection state tables (TCP sessions, NAT mappings, VPN tunnels) over
      a dedicated HA link. Without this, existing connections are dropped when the VIP moves.

---

## 4. Cloud Fabric and SDN Prerequisites (STACKIT)

- [ ] **Allowed Address Pairs (port security bypass)**: The cloud network ports (vNICs) of _both_
      cluster nodes must be configured with an Allowed Address Pair entry for the VIP. Without this,
      OVN port security silently discards every GARP packet the appliance sends, because the source
      MAC/IP combination is not registered on that port.

- [ ] **Virtual dummy port for the VIP**: The cloud floating IP (public IP) must be associated
      with a dedicated virtual/dummy port that represents the internal VIP — never statically bound
      to the vNIC of node 1 or node 2. This allows the cloud router to follow the ARP table entry
      that changes with each GARP, routing inbound traffic to whichever node is currently active.
