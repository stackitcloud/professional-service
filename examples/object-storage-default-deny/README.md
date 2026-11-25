<!-- tags: object-storage, s3, access-control, iam, security, terraform -->

# Object Storage Default Deny for Same-Project Credentials

## Overview

Demonstrates how to enforce a **default-deny** access policy on a STACKIT Object Storage bucket for all credentials within the same STACKIT project and control permissions exclusively through S3 bucket policies.

By default, all Object Storage credentials created in the same STACKIT project are granted full access (`s3:*`, "default allow") to every bucket in that project. Because S3 policy evaluation treats an explicit `Deny` as overriding any `Allow`, an S3 bucket policy must use `Effect: "Deny"` with `NotPrincipal` and `NotAction` to revoke the project-wide default allow and restrict each credential group to least-privilege permissions.

This example creates (all within the **same STACKIT project**):

- An Object Storage bucket
- Four credentials groups and credentials in the same project:
  - **Admin** (`admin-credentials`) — full `s3:*` access to manage the bucket and its bucket policy
  - **Write-only** (`write-only-credentials`) — allowed only `s3:PutObject` and `s3:AbortMultipartUpload`
  - **Read-only** (`read-only-credentials`) — allowed only `s3:GetObject` and `s3:ListBucket`
  - **Unprivileged** (`unprivileged-credentials`) — not listed in the policy and therefore completely denied by default
- An S3 bucket policy (`aws_s3_bucket_policy`) that denies all other project credentials by default (`NotPrincipal`) and restricts each allowed credential group to its permitted actions (`NotAction`)

## How the S3 Policy Works

In STACKIT Object Storage, credentials in the same project inherit an implicit project-level `Allow` for `s3:*`. Simply adding `Effect: "Allow"` statements in a bucket policy does not remove that project-wide permission. Instead, the bucket policy uses three layers:

1. **Default Deny (`NotPrincipal`)**: Explicitly denies `s3:*` on the bucket and its objects to every principal whose URN is **not** in the allow-list (`admin`, `write`, `read`). Any other credential in the same project (such as `unprivileged-credentials` or future credentials) has zero access.
2. **Action Restriction (`NotAction`)**: Because `write` and `read` are excluded from the `NotPrincipal` deny and still hold the project-level default allow, dedicated `Deny` + `NotAction` statements strip away every S3 action except their intended permissions (`PutObject`/`AbortMultipartUpload` for `write`, `GetObject`/`ListBucket` for `read`).
3. **Explicit Allow (`Action`)**: Explicitly grants the intended actions to `admin`, `write`, and `read`.

## Architecture

```
STACKIT Project
├── Object Storage Bucket
│   └── S3 Bucket Policy (Default Deny via NotPrincipal + Least Privilege via NotAction)
├── Admin Credentials Group        → Full access (s3:*)
├── Write-Only Credentials Group   → Write only (s3:PutObject, s3:AbortMultipartUpload)
├── Read-Only Credentials Group    → Read only (s3:GetObject, s3:ListBucket)
└── Unprivileged Credentials Group → Denied all actions (Default Deny)
```

## Prerequisites

- Terraform >= 1.5.0
- A STACKIT service account with permissions to manage Object Storage resources in the project

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

## Verification

Run the included test script (requires [`s5cmd`](https://github.com/peak/s5cmd)) to verify the effect of the S3 bucket policies across all four credential groups (`write-only`, `read-only`, `unprivileged`, and `admin`):

```bash
chmod +x test-policies.sh
./test-policies.sh
```

The script automatically reads the credentials and endpoint from Terraform/OpenTofu outputs, tests `PutObject` (`cp`), `ListBucket` (`ls`), `GetObject` (`cat`), and `DeleteObject` (`rm`) for each credential group, and cleans up the test objects afterward.

## Cleanup

Ensure the bucket is empty before destroying the infrastructure:

```bash
terraform destroy
```
