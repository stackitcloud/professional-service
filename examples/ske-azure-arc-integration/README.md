<!-- tags: ske, azure, arc, kubernetes, hybrid, multi-cloud -->

# STACKIT SKE Azure Arc Integration

This repository contains Terraform and CLI steps to connect a **STACKIT SKE cluster** to **Azure Arc**.

## Prerequisites

- Azure CLI installed and authenticated (`az login`)
- Terraform installed
- STACKIT Project & Service Account configured

## Setup Guide

### 1. Provision Infrastructure

Deploy the SKE cluster and an Azure Resource Group to host the Arc connection:

```bash
terraform init
terraform apply
```

### 2. Connect to Azure Arc

Run the following commands to register required Azure providers and connect the cluster:

```bash
# Register Azure Arc providers
az extension add --name connectedk8s
az provider register --namespace Microsoft.Kubernetes
az provider register --namespace Microsoft.KubernetesConfiguration
az provider register --namespace Microsoft.ExtendedLocation

# Export SKE Kubeconfig
terraform output -raw kubeconfig > .kubeconfig

# Connect cluster to Azure Arc
az connectedk8s connect \
  --name "stackit-$(terraform output -raw cluster_name)" \
  --resource-group "$(terraform output -raw azure_resource_group)" \
  --location "$(terraform output -raw azure_location)" \
  --kube-config .kubeconfig
```

## References

- [Azure Arc Quickstart](https://learn.microsoft.com/en-us/azure/azure-arc/kubernetes/quickstart-connect-cluster?tabs=azure-cli)
