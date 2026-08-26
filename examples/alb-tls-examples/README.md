<!-- tags: alb, tls, certificate, load-balancer, lets-encrypt, iaas, ske -->

# alb-tls-examples

A collection of STACKIT Application Load Balancer (ALB) showcases with different TLS strategies — from self-signed to Let's Encrypt, from a single VM to Kubernetes.

Each subfolder is a self-contained, runnable Terraform showcase with its own README, state, and variables. The showcases are intentionally independent — no shared state, no shared modules.

---

## Overview

```
alb-tls-examples/
│
├── vm-alb-self-signed-cert/        ← Starting point: 1 VM + ALB + Self-Signed Cert (Terraform)
├── vm-alb-certbot-letsencrypt/     ← Production: VM + ALB + Let's Encrypt via certbot + ACME DNS-01
└── alb-k8s/                        ← Kubernetes: SKE + cert-manager + Let's Encrypt
```

---

## Showcase Comparison

|                           | `vm-alb-self-signed-cert`    | `vm-alb-certbot-letsencrypt`  | `alb-k8s`                     |
| ------------------------- | ---------------------------- | ----------------------------- | ----------------------------- |
| **Goal**                  | Getting started / quickstart | Production-grade              | Kubernetes path               |
| **Certificate**           | Self-signed (Terraform)      | Let's Encrypt (certbot)       | Let's Encrypt (cert-manager)  |
| **Backend**               | 1 VM + Docker nginx          | 1 VM + Docker nginx           | SKE cluster + nginx Pod       |
| **Terraform**             | Yes                          | Yes (Phase 1)                 | Yes                           |
| **Docker**                | Yes (nginx)                  | Yes (nginx + certbot)         | No                            |
| **Kubernetes**            | No                           | No                            | Yes (SKE)                     |
| **Auto-renewal**          | No (re-apply)                | Yes (cron on VM)              | Yes (cert-manager)            |
| **External dependencies** | None                         | Let's Encrypt, DNS delegation | Let's Encrypt, DNS delegation |
| **Time to HTTPS**         | ~5 min                       | ~20 min                       | ~20 min                       |

---

## Learning Path

**1. [`vm-alb-self-signed-cert/`](vm-alb-self-signed-cert/README.md)**

- How does a STACKIT ALB work?
- How is a TLS certificate attached to the ALB?
- How does the ALB terminate HTTPS and forward to a backend?

**2. [`vm-alb-certbot-letsencrypt/`](vm-alb-certbot-letsencrypt/README.md)**

- How do I replace a self-signed cert with a trusted one?
- How does the ACME DNS-01 challenge work with STACKIT DNS?
- How does automatic certificate renewal work via certbot in Docker?

**3. [`alb-k8s/`](alb-k8s/README.md)**

- How does the same work on Kubernetes?
- How do cert-manager, Ingress, and STACKIT SKE interact?

---

## Common Prerequisites

| Requirement             | Details                                                  |
| ----------------------- | -------------------------------------------------------- |
| STACKIT account         | Access to the STACKIT Portal                             |
| Terraform               | >= 1.5.7 (recommended: use `tfenv`)                      |
| STACKIT CLI             | For image UUIDs, project IDs, debugging                  |
| SSH key pair            | Ed25519 or RSA — only the public key goes into Terraform |
| STACKIT service account | JSON key with the roles required for the showcase        |
| STACKIT Object Storage  | For the S3-compatible Terraform remote state backend     |

### Create a service account

```bash
stackit iam service-account create \
  --project-id <project-id> \
  --name "tf-workshop-sa"

mkdir -p keys
stackit iam service-account key create \
  --project-id <project-id> \
  --service-account-email <sa-email> \
  --output-format json > keys/sa-key.json
```

### Useful CLI commands

```bash
# List available Debian 12 images
stackit image list --all --project-id <project-id>

# List available machine types
stackit server machine-type list --project-id <project-id>

# List projects
stackit project list

# Find your egress IP (for admin_cidr)
curl -s https://ifconfig.schwarz
```

---

## Showcase Descriptions

### [`vm-alb-self-signed-cert/`](vm-alb-self-signed-cert/README.md)

**Introductory showcase — recommended starting point.**

A single Debian 12 VM with Docker and `nginx:alpine` behind a STACKIT Application Load Balancer. The TLS certificate is self-signed and fully managed by Terraform — no external tools, no DNS delegation required.

```
Internet → ALB (HTTPS :443, Self-Signed Cert) → VM (Docker nginx :80)
```

- Terraform generates RSA key + self-signed cert (`hashicorp/tls` provider)
- Terraform uploads the certificate via `stackit_alb_certificate`
- The ALB terminates TLS, the VM receives plain HTTP
- STACKIT DNS Zone + A-Record created as part of the same apply

**When to use:** When you want to understand the ALB + TLS mechanism without extra complexity.

---

### [`vm-alb-certbot-letsencrypt/`](vm-alb-certbot-letsencrypt/README.md)

**Production showcase with automatic certificate renewal.**

Same infrastructure as `vm-alb-self-signed-cert`, but with a full ACME pipeline on the VM. Terraform provisions the ALB without a certificate; a certbot Docker container then issues and renews Let's Encrypt certificates via DNS-01 challenge.

```
Phase 1 (Terraform):   VM + ALB (HTTP + target) + DNS Zone
Phase 2 (certbot):     ACME DNS-01 → Let's Encrypt Cert → ALB HTTPS listener
Phase 3+ (cron):       Monthly automatic renewal
```

- Requires a delegated DNS zone (set NS records at your registrar)
- `lifecycle { ignore_changes = [listeners] }` allows certbot to update the ALB outside of Terraform

**When to use:** When you want a production-realistic, fully automated certificate lifecycle.

---

### [`alb-k8s/`](alb-k8s/README.md)

**Kubernetes showcase with STACKIT SKE and cert-manager.**

Terraform creates a STACKIT Kubernetes Engine (SKE) cluster. nginx is deployed as a Kubernetes Deployment, the STACKIT NLB acts as the ingress point. cert-manager handles automatic certificate management via Let's Encrypt DNS-01 with STACKIT DNS.

```
Internet → STACKIT NLB (L4, auto) → Traefik IC (L7, in-cluster) → nginx Pod
                                     cert-manager (Let's Encrypt DNS-01)
```

**When to use:** When you want to show how TLS works on a Kubernetes-based platform with STACKIT.

---

## Common Architecture Principles

**TLS termination at the load balancer**
The ALB (or NLB + Traefik IC on Kubernetes) terminates HTTPS. Backend VMs or pods only receive plain HTTP on port 80 over the private network.

**Infrastructure as Code**
All resources are managed via Terraform. No manual clicking in the portal.

**Gitignored secrets**
`terraform.tfvars`, `backend.conf`, and `keys/` are gitignored in every showcase. No secret ends up in the repository.

---

## References

- [STACKIT Terraform Provider](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs)
- [STACKIT Developer Documentation](https://docs.stackit.cloud)
- [STACKIT CLI](https://github.com/stackitcloud/stackit-cli)
- [hashicorp/tls Provider](https://registry.terraform.io/providers/hashicorp/tls/latest/docs)
- [cert-manager](https://cert-manager.io/docs/)
