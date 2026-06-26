# Phase 0: Bootstrap

This module provisions the STACKIT PostgreSQL Flex instance, the `terraform_state` database, and the dedicated `tf_state_user`. Its state is kept locally (or in an independent CI/CD backend) to prevent dependency conflicts.

## Implementation Steps

1. Initialize Terraform with the default local backend:

   ```sh
   terraform init
   ```

2. Provision the PostgreSQL Flex resources:

   ```sh
   terraform apply
   ```

3. Extract the generated PostgreSQL connection string from the Terraform outputs. This URI is required to configure the remote backend in the next phase.

   ```sh
   terraform output -raw pg_connection_uri
   ```
