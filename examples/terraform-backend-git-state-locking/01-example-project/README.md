# Phase 1: Example Infrastructure

This module contains the core infrastructure configuration. Terraform runs through the [`terraform-backend-git`](https://github.com/plumber-cd/terraform-backend-git) wrapper, which stores the encrypted state and the state locks in the STACKIT Git repository prepared in [Phase 0](../00-bootstrap/).

## How the wrapper works

Instead of `terraform <command>` you run:

```sh
terraform-backend-git git terraform <command>
```

For every call, the wrapper:

1. reads the repository, branch and state path from the environment variables `TF_BACKEND_GIT_GIT_REPOSITORY`, `TF_BACKEND_GIT_GIT_REF` and `TF_BACKEND_GIT_GIT_STATE`,
2. starts a local HTTP backend on `127.0.0.1:6061`,
3. writes the matching `backend "http"` block to `git_http_backend.auto.tf`,
4. runs Terraform with all given arguments,
5. deletes `git_http_backend.auto.tf` (only if Terraform succeeded) and stops the backend again.

There is no backend block in the code and no process to keep running. Per Terraform directory you only need a `.env` with the Git credentials, the sops/age settings and a unique state path (`TF_BACKEND_GIT_GIT_STATE`). Load the `.env` of the current directory before every call.

To save typing, define an alias:

```sh
alias tfg='terraform-backend-git git terraform'
```

## Prerequisites

- The Git instance, the state repository and an access token, see [`00-bootstrap/`](../00-bootstrap/README.md).
- [`terraform-backend-git`](https://github.com/plumber-cd/terraform-backend-git/releases) in your `PATH`:

  ```sh
  VERSION=v0.1.12
  ASSET=terraform-backend-git-darwin-arm64
  curl -fsSLO "https://github.com/plumber-cd/terraform-backend-git/releases/download/${VERSION}/${ASSET}"
  curl -fsSLO "https://github.com/plumber-cd/terraform-backend-git/releases/download/${VERSION}/SHA256SUMS"
  grep " ${ASSET}\$" SHA256SUMS | shasum -a 256 -c -
  install -m 0755 "${ASSET}" /usr/local/bin/terraform-backend-git
  ```

  [Reference](https://github.com/plumber-cd/terraform-backend-git#from-release)

- [`age`](https://github.com/FiloSottile/age) to create an encryption key pair (`brew install age`).

## Implementation Steps

1. Create an age key pair. The public key (`age1...`) is printed and also stored in the file. Keep the private key out of version control and back it up: without it, the state cannot be decrypted.

   ```sh
   mkdir -p ~/.config/sops/age
   age-keygen -o ~/.config/sops/age/keys.txt
   ```

2. Create a `.env` in the example root from `.env.example` with your Git user, access token and the age public keys

   ```sh
   source .env
   ```

   > **Important:** without `TF_BACKEND_HTTP_ENCRYPTION_PROVIDER` and `TF_BACKEND_HTTP_SOPS_AGE_RECIPIENTS`, `terraform-backend-git` stores the state in plain text in the Git repository. Load the settings before every Terraform session.

3. Create a `terraform.tfvars` from [`terraform.tfvars.example`](terraform.tfvars.example).

4. Initialize Terraform through the wrapper:

   ```sh
   tfg init
   ```

5. Provision the infrastructure:

   ```sh
   tfg plan
   tfg apply
   ```

6. Check the state file in the repository after the first `apply`: it must contain a `sops` section and encrypted values (`ENC[AES256_GCM,...]`).

## What you see in the repository

- On `main`: one commit per state change
- While Terraform runs: a branch `locks/01-example-project/terraform.tfstate.json` with the lock metadata (who, when, which operation). It is deleted when the run ends.

## Using it in CI

Run the same commands in the pipeline job: install `terraform-backend-git`, provide `GIT_USERNAME`, `GIT_PASSWORD`, `TF_BACKEND_HTTP_ENCRYPTION_PROVIDER`, `TF_BACKEND_HTTP_SOPS_AGE_RECIPIENTS` and the CI's age private key as `SOPS_AGE_KEY` from the secret store, then call `terraform-backend-git git terraform init|plan|apply`. Add the CI's age public key to the recipients list and the egress IPs of the runners to `git_acl`.
