# Velero Backup for STACKIT Kubernetes Engine

## Overview

This example deploys [Velero](https://velero.io/) on a STACKIT Kubernetes Engine (SKE) cluster using STACKIT Object Storage as the backup backend.

It provisions the SKE cluster, a dedicated S3 bucket with credentials, and installs Velero via Helm with a nightly full-cluster backup schedule preconfigured.

Reference: [STACKIT Docs — Backup your cluster with Velero](https://docs.stackit.cloud/products/runtime/kubernetes-engine/how-tos/backup-your-cluster-with-velero/)

## Usage

```bash
terraform init
terraform plan
terraform apply
```

Verify Velero is healthy after apply:

```bash
stackit ske kubeconfig create --cluster-name velero-demo --project-id <project-id>
velero backup-location get  # should show status: Available
```

## Notes

- Volume snapshots are disabled. PVC data backup requires enabling file-system backup (Restic/Kopia) — see the [Velero FSB docs](https://velero.io/docs/latest/file-system-backup/).
- The Object Storage bucket name is globally unique by design (random suffix). Do not rename it after creation.
- S3 credentials are stored in a Kubernetes secret and in Terraform state. Use STACKIT Secrets Manager for production hardening.
