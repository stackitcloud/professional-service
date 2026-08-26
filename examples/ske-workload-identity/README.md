<!-- tags: ske, workload-identity, iam, kubernetes, oidc, service-account -->

# SKE Workload Identity

## Overview

Demonstrates STACKIT Workload Identity on SKE — pods authenticate to the STACKIT API using short-lived OIDC tokens instead of static service account keys.

The [stackit-pod-identity-webhook](https://github.com/stackitcloud/stackit-pod-identity-webhook) (pre-installed on SKE) intercepts pod creation and injects a projected JWT token and the `STACKIT_SERVICE_ACCOUNT_EMAIL` environment variable. The STACKIT SDK picks these up automatically and exchanges the Kubernetes token for a scoped STACKIT access token.

Reference: [STACKIT Docs — Workload Identity](https://docs.stackit.cloud/products/runtime/kubernetes-engine/how-tos/workload-identity/)

## Usage

```bash
terraform init
terraform apply
```

Terraform creates the SKE cluster, the STACKIT service account, and configures the OIDC federation trust automatically. No manual portal steps required.

## Verify

```bash
stackit ske kubeconfig create --cluster-name <cluster-name> --project-id <project-id> -y

# Watch the demo job — it calls the STACKIT SKE API using the injected identity
kubectl logs job/workload-identity-demo -n workload-identity-demo
```

A successful run prints the available SKE Kubernetes versions, proving the token exchange worked.

## Assigning permissions

The created STACKIT service account has no roles by default. Add project-level role assignments in Terraform via `stackit_authorization_project_role_assignment` to match your workload's actual API access requirements.
