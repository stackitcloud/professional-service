# Object Storage Write-Only Access with Object Lock

## Overview

Demonstrates how to create a STACKIT Object Storage bucket with Object Lock enabled, configure write-only access credentials, and store all credentials securely in STACKIT Secrets Manager using the HashiCorp Vault provider.

This example creates:

- An Object Storage bucket with Object Lock (compliance mode)
- Three sets of credentials: admin, write-only, and read-only
- A dedicated sub-project for credentials isolation
- A STACKIT Secrets Manager instance to store the credentials
- An S3 bucket policy that restricts the write-only credentials to `PutObject`, `AbortMultipartUpload`, and `PutObjectRetention` actions

## Prerequisites

- Terraform >= 1.5.0
- A STACKIT service account with permissions to create Object Storage, Secrets Manager, and Resource Manager resources

## Setup

### 1. Configure variables

Copy the example variables file and adjust values:

```bash
cp terraform.tfvars.example terraform.tfvars
```

### 2. Initialize and apply

```bash
terraform init
terraform plan
terraform apply
```

## Architecture

```
STACKIT Project
├── Object Storage Bucket (Object Lock enabled)
├── Credentials Sub-Project
│   ├── Admin Credentials Group (full access)
│   ├── Write-Only Credentials Group (PutObject only)
│   └── Read-Only Credentials Group
└── Secrets Manager Instance
    ├── s3-credentials/admin
    ├── s3-credentials/write
    └── s3-credentials/read
```

## Cleanup

```bash
terraform destroy
```
