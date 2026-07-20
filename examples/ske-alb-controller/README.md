# SKE Application Load Balancer Controller (L7 Ingress)

Deploys the [STACKIT ALB Controller](https://github.com/stackitcloud/application-load-balancer-controller) into an SKE cluster and verifies it end-to-end: the controller provisions a managed L7 ALB from an `IngressClass`/`Ingress`, including TLS via the ALB certificate service.

> Installing the controller manually like this is an **interim solution** until the SKE-ALB integration is available as a native SKE extension. Once released, prefer the extension over this deployment.

## What gets created

- SNA + project + node network — required so the node network ID for the controller config (`cloud.yaml`) is known.
- SKE cluster ([`test-ske`](../../modules/test-ske) module) deployed into that network.
- Controller service account with a least-privilege custom role (`alb.loadbalancer.*`, `alb.targetpool.replace`, `alb.certificateservice.certificate.*`) and a rotated key.
- ALB controller in `kube-system` (Terraform port of the PR's kustomize deployment).
- Test workload in `alb-demo`: IngressClass (provisions the ALB), demo app with NodePort service, self-signed TLS secret, HTTP+HTTPS Ingress.

The apply blocks until the ALB reports a public IP on the Ingres, a successful apply proves the integration works.

## Usage

Requires a service account key with org-level permissions (creates SNA + project).

```bash
terraform init
terraform apply
```

Verify (no DNS needed, uses `curl --resolve`):

```bash
eval "$(terraform output -raw test_http_command)"
eval "$(terraform output -raw test_https_command)"
```

## Cleanup

```bash
terraform destroy
```
