# SKE Kubernetes Ephemeral Kubeconfig

## Overview

Deploy an SKE cluster and use an **ephemeral kubeconfig** to authenticate the Kubernetes and Helm Terraform providers without ever writing credentials to state.

## Usage

### Prerequisites

- Terraform ≥ 1.10
- A STACKIT project with SKE enabled
- A STACKIT service account key with permissions to manage SKE clusters

### Configure variables

Copy `prod.auto.tfvars` and fill in your values:

```hcl
project_id                       = "<your-stackit-project-id>"
stackit_service_account_key_path = "/path/to/sa-key.json"
```

### Apply

```bash
terraform init
terraform apply
```

The ephemeral kubeconfig is fetched automatically during the run no manual `kubectl` setup required.
