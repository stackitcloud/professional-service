<!-- tags: ske, kubernetes, terraform, provider, kubeconfig -->

# Terraform SKE Kubernetes Provider Integration

## Overview

Demonstrates how to use the HashiCorp `kubernetes` provider to manage resources inside a SKE cluster.

## Security consideration

This example uses a `stackit_ske_kubeconfig` managed resource to configure the Kubernetes provider. Terraform stores managed resources in state, so the kubeconfig — including client certificates and keys — persists in `terraform.tfstate` for the lifetime of the workspace.

If you are on Terraform >= 1.10, consider the [ske-kubernetes-ephemeral-kubernetes-provider](../ske-kubernetes-ephemeral-kubernetes-provider) example instead. It uses an ephemeral resource to fetch the kubeconfig in memory for the duration of each run and discard it on exit — nothing lands in state. The feature is currently experimental in the STACKIT provider.
