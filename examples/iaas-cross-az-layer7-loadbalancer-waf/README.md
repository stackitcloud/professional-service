# IaaS cross AZ Layer 7 Loadbalancer

## Overview

A classic highly-available architecture: provisioning multiple VMs across different Availability Zones (AZs) and putting them behind a STACKIT L7 Load Balancer. This example also includes a Web Application Firewall (WAF) configuration to secure the backend workloads against malicious traffic.

## WAF Resources

WAF resources are managed natively by the STACKIT Terraform provider (>= 0.110.0):

| Resource                            | Purpose                                               |
| ----------------------------------- | ----------------------------------------------------- |
| `stackit_alb_waf_managed_rule_set`  | Activates the OWASP Core Rule Set (CRS)               |
| `stackit_alb_waf_custom_rule_group` | Defines custom block rules                            |
| `stackit_alb_waf_configuration`     | Attaches both rule sets to the load balancer listener |

> **Note:** WAF resources are currently in beta. `enable_beta_resources = true` must be set in the `stackit` provider block.

## Testing the WAF

Once `terraform apply` completes, extract the public IP of your Load Balancer from the Terraform outputs:

```bash
export ALB_IP=$(terraform output -raw alb_external_address)
```

Use `curl` to trigger the custom block rules. Both commands should return HTTP `403 Forbidden`.

**Test 1 — Query parameter**

```bash
curl -k -I -X GET "https://${ALB_IP}/?waf_test=trigger"
```

**Test 2 — Custom HTTP header**

```bash
curl -k -I -H "X-WAF-Test: trigger" "https://${ALB_IP}/"
```
