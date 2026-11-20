# Phase 0: Bootstrap

This module provisions a STACKIT project and the STACKIT Git instance that hosts the Terraform state repository. Its own state is kept locally.

## Implementation

   ```sh
   terraform init
   terraform plan
   terraform apply
   ```

## Prepare the state repository

The STACKIT Terraform provider manages the Git instance, not its content. Do the following once in the web UI of the instance:

1. **Create a private repository** for state files, e.g. `<owner/organization>/tf-state`
   - Enable **Initialize repository** so that the default branch `main` exists.
2. **Protect the state branch:** _Settings → Branches → Add new rule_ for `main`:
   - **Push:** set to _Enable push_, or to _Whitelist restricted push_ with the users that run Terraform. A new rule defaults to _Disable push_, which rejects every state update with `pre-receive hook declined`.
   - **Force push:** disabled.
   - **Require signed commits:** disabled, `terraform-backend-git` does not sign its commits.

   Leave `locks/*` unprotected, the backend creates and deletes these branches.
3. **Create an access token** for the user that runs Terraform with the scope `repository: read and write`. For CI, use a dedicated technical user with access to the state repository only.