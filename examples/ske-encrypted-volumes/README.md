<!-- tags: ske, encryption, kms, block-storage, kubernetes, csi -->

# Encrypted Volumes for SKE

> ⚠️This example assumes that your project or organization has been enabled for a preview version of the STACKIT CSI Driver. If you wish to use encrypted volumes, please contact your account manager.

## Overview

This guide demonstrates how to roll out an encrypted storage class for SKE using the STACKIT Key Management Service (KMS). To achieve this, we use a **Service Account Impersonation** (Act-As) pattern. This allows the internal SKE service account to perform encryption and decryption tasks on behalf of a user-managed service account that has been granted access to your KMS keys.

## Prerequisites

- Terraform 1.10 or later
- STACKIT provider 0.117.0 or later
- An authenticated `stackit` CLI (`stackit auth login` or `stackit auth activate-service-account`) and `kubectl` for the verify step

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform apply
```

`terraform.tfvars` needs `stackit_project_id`. Without `stackit_service_account_key_path`, the provider falls back to `STACKIT_SERVICE_ACCOUNT_KEY_PATH`, then to its credentials file `$HOME/.stackit/credentials.json` ([provider authentication](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs)).

## Verify

Check that the claim is bound and the pod is running:

```bash
eval "$(terraform output -raw kubeconfig_command)"
kubectl config use-context "$(terraform output -raw ske_cluster_name)"
kubectl get pvc test-encryption-pvc
kubectl get pod encrypted-volume-test
```
