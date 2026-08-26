<!-- tags: iam, rbac, access-control, service-account, terraform -->

# IAM Custom Roles

## Overview

Demonstrates how to create and assign STACKIT custom IAM roles to a service account using Terraform.

Custom roles let you grant exactly the permissions a workload needs — nothing more. Unlike built-in roles (e.g. `editor`, `viewer`), custom roles are scoped to a specific set of API permissions and are defined at project level.

This example creates:

- A service account with an automatically rotating key (80-day rotation, 90-day TTL)
- A custom role with read-only access to SKE and Object Storage
- A role assignment binding the custom role to the service account

## Usage

```bash
terraform init
terraform apply
```

Retrieve the generated key to authenticate as the service account:

```bash
terraform output -json service_account_key
```

## Key rotation

The `time_rotating` resource triggers key recreation every 80 days. Running `terraform apply` after the rotation window elapses will replace the key before it expires.

## Extending the role

Add or remove entries in the `permissions` list of `stackit_authorization_project_custom_role.readonly` to adjust access. The full list of available permissions can be found in the [STACKIT IAM documentation](https://docs.stackit.cloud/products/identity-and-access-management/).

A service account can hold multiple role assignments — create additional `stackit_authorization_project_custom_role` and `stackit_authorization_project_role_assignment` resources to compose access from several fine-grained roles.
