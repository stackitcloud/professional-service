# Public Web UI Access via the Management Interface

Per [`docs/initial-setup.md`](initial-setup.md), the OPNsense web UI is reached over **WAN** for initial
setup, then handed over to the **MGMT** interface (OPT1, `10.28.0.36`, reachable via `mgmt_public_ip`) as the
sole long-term admin channel — kept separate from WAN/LAN so that firewall/routing policy for actual traffic
never has to make an exception for management access. That separation causes one non-obvious failure mode
worth fixing (and understanding) before you switch admin access over to MGMT and lock WAN back down.

## The problem: MGMT ingress, WAN egress

Symptom: `https://<mgmt_public_ip>/` times out or resets, even though the MGMT security group
(`firewall-mgmt-sg` in [`030-network.tf`](../001-hub-project/030-network.tf)) allows TCP/443 from your
`mgmt_ip_range`, and the OPNsense-side rule for the OPT1/MGMT interface looks correct.

**Root cause:** OPNsense's firewall evaluates _floating_ rules before interface-specific rules. If a floating
rule permits TCP/443 (a common pattern when opening HTTPS broadly, e.g. for reverse-proxied services), it
can match the inbound MGMT WebUI request first — and floating rules use the system's default gateway (WAN)
for routing decisions unless told otherwise. The reply then leaves via WAN instead of MGMT. Because the
request came in on MGMT but the reply goes out on WAN, the client sees a broken TCP handshake and the
connection fails. Traffic follows: **MGMT ingress → WAN egress**, when it needs to be **MGMT ingress → MGMT
egress**.

This is easy to trigger by accident: any floating TCP/443 rule added for another purpose will shadow the
WebUI unless it's scoped away from MGMT traffic.

## The fix

Apply both of the following (belt and suspenders — either alone is enough, together they prevent regressions):

1. **Pin the OPT1/MGMT interface rule's reply path.** Since OPT1 was assigned with DHCP
   ([`docs/initial-setup.md`](initial-setup.md) § 2), OPNsense auto-creates a matching Gateway object —
   check its exact name under **System → Gateways** (typically `OPT1_DHCP`). Then, on the OPT1-interface
   rule permitting TCP/443 (**Firewall → Rules → OPT1**), open the rule's **Advanced** section, set
   **Gateway**/**Reply-to** to that gateway explicitly. This forces OPNsense to add a `reply-to` directive to
   the generated pf rule, so replies to MGMT-received connections always leave via OPT1 regardless of the
   default route.
2. **Scope or remove the conflicting floating rule.** Under Firewall → Rules → Floating, either delete the
   broad TCP/443 rule if nothing needs it, or restrict its **Interface** selection so it excludes OPT1
   (`WAN, LAN` instead of `any`). Floating rules are evaluated in order and match multiple interfaces by
   default — the safest posture is to never let a floating rule implicitly cover MGMT.

Rule of thumb: interface-specific rules with an explicit gateway are self-defending against future floating
rules added for unrelated reasons; relying on floating-rule scoping alone is fragile because the next person
to add a rule may not know about the MGMT interface's routing requirement.

## Switching from WAN to MGMT

Once this fix is applied and OPT1/MGMT's web UI is reliably reachable on its own public IP:

1. Confirm `https://<mgmt_public_ip>/` loads and logs in correctly.
2. Review **Firewall → Rules → WAN** and remove whatever admin access (443/22) you opened there for the
   initial setup in `docs/initial-setup.md` — WAN shouldn't carry admin traffic long-term. What's actually
   on WAN depends on what you (or the image defaults) put there, so this is a manual review, not a fixed
   rule to delete by name.
