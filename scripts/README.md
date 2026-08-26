# Scripts

Helper scripts for working with STACKIT services.

> **Warning:** These scripts can perform destructive or wide-reaching operations against your STACKIT account (e.g. deleting volumes, overwriting kubeconfigs, writing secrets). Review each script and understand what it does before running it.

## Overview

| Script                                                                             | Purpose                                                                                                               | Required tools                     | Tags                                                        |
| ---------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- | ---------------------------------- | ----------------------------------------------------------- |
| [`s3-ssec.sh`](#s3-ssec)                                                           | Upload and Download Files to a STACKIT S3 Bucket (Object Storage) and enable Server Side Encryption with Custom Keys. | `awscli`, `python3-awscrt`         | `object-storage, s3, encryption, ssec`                      |
| [`check-stackit-ip.sh`](#check-stackit-ipsh)                                       | Check whether a given IP address belongs to STACKIT's public IP ranges.                                               | `stackit`, `jq`, `grepcidr`        | `networking, ip, iaas`                                      |
| [`check-terraform-numbered-files.sh`](#check-terraform-numbered-filessh)           | Verify that all Terraform files in an example follow the 3-digit numeric prefix naming convention.                    | (none — used as a pre-commit hook) | `terraform, lint, ci`                                       |
| [`create-kubeconfig-multiple-projects.sh`](#create-kubeconfig-multiple-projectssh) | Generate kubeconfig entries for every SKE cluster across one or more STACKIT projects.                                | `stackit`, `yq`                    | `ske, kubernetes, kubeconfig, multi-project`                |
| [`delete-unused-volumes.sh`](#delete-unused-volumessh)                             | Delete all STACKIT volumes whose status is `AVAILABLE` (i.e. not attached).                                           | `stackit`, `yq`                    | `iaas, volume, compute`                                     |
| [`list-project-resources.sh`](#list-project-resourcessh)                           | Render a Markdown inventory of resources (DNS, SKE, databases, storage, …) for one or more STACKIT projects.          | `stackit`, `jq`                    | `dns, ske, dbaas, object-storage, inventory, multi-project` |
| [`ske-show-versions.sh`](#ske-show-versionssh)                                     | Print overview of SKE cluster Kubernetes versions and nodepool image versions, marking deprecated versions.           | `stackit` (>= 0.59.0), `jq`, `awk` | `ske, kubernetes, versions`                                 |
| [`smctl.sh`](#smctlsh)                                                             | Unified CLI wrapper around HashiCorp Vault for the STACKIT Secret Manager (KV v2), think `kubectl` for secrets        | `vault`, `jq`                      | `secrets-manager, vault, secrets`                           |
| [`vault-migrate.sh`](#vault-migratesh)                                             | Migrate secrets between two Vault instances using the KV v2 API (supports userpass and LDAP for source).              | `vault`, `jq`                      | `secrets-manager, vault, secrets, migration`                |

---

## `s3-ssec.sh`

### STACKIT S3 Object Storage SSE-C Automation Script

This script automates secure Server-Side Encryption with Customer-Provided Keys (SSE-C) using the native aws-cli tool against STACKIT Object Storage.

### Why This Script Exists

Standard S3 tools (s3cmd, rclone) often fail during SSE-C operations on Linux due to a Binary-to-Text Encoding Mismatch when passing keys via the command line. Furthermore, third-party tools frequently throw false alarms regarding data corruption because the server-side encrypted file's MD5 hash (ETag) no longer matches the local unencrypted file's hash.

The Solution: This script utilizes the official aws-cli along with a binary key file reference (fileb://). This approach bypasses shell encoding issues entirely. The AWS CLI natively understands SSE-C protocols, managing hash verifications and setting all necessary headers automatically.

### Prerequisites

AWS CLI installed:

```bash
sudo apt install awscli -y  # Debian/Ubuntu
sudo dnf install aws-cli -y # RHEL/CentOS
```

Permissions: Root access (or appropriate sudo permissions) to store the encryption key safely under /root/ssec.key.

### Setup & Configuration

Open the script and fill in your STACKIT credentials and bucket name under the configuration block:

```bash
export AWS_ACCESS_KEY_ID="YOUR_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="YOUR_SECRET_ACCESS_KEY"
BUCKET="s3://your-bucket-name"
```

Note: On its very first run, the script will automatically generate a cryptographically secure 32-byte binary AES key using openssl and lock down its file permissions (chmod 400).

### Usage

Make the script executable:

```bash
chmod +x s3-ssec.sh
```

1. Uploading a File (Encrypted)
   To upload and encrypt a local file, pass upload followed by the file path:

```bash
./s3-ssec.sh upload /path/to/local/file.txt
```

The file will be encrypted on the fly by the STACKIT storage backend using your generated key.

2. Downloading a File (Decrypted)
   To download and decrypt an object from the bucket, pass download followed by the remote object name:

```bash
./s3-ssec.sh download file.txt
```

The script will fetch the file, provide the required key headers, and save the decrypted file locally as ./file.txt_downloaded.

Security Warning ⚠️
Backup your Key: If you lose the /root/ssec.key file, any data encrypted with it in the bucket cannot be recovered.

Credential Rotation: Never commit the script to public repositories while it contains live AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY strings. Use environment variables or secret managers in production environments.

---

## `check-stackit-ip.sh`

Looks up the STACKIT public IP ranges (via `stackit curl https://iaas.api.eu01.stackit.cloud/v1/networks/public-ip-ranges`) and tells you whether a given IP falls inside any of them. Useful to verify whether a public-facing address actually originates from STACKIT.

Exits `0` if the IP is found, `1` otherwise.

### Example

```bash
# Check a single IP
./check-stackit-ip.sh 45.129.40.1
```

Sample output:

```
Fetching STACKIT IP ranges...
Found: 45.129.40.1 is in the range 45.129.40.0/22
```

---

## `create-kubeconfig-multiple-projects.sh`

Logs in to STACKIT, iterates over a hard-coded list of project IDs, lists all SKE clusters in each project and writes a 60-day kubeconfig for every cluster into a single kubeconfig file.

Edit the `projects=( ... )` array near the bottom of the script before running.

### Flags

- `-f, --filepath <path>` — destination kubeconfig path (default: `$HOME/.kube/config`).
- `-h, --help` — show usage.

### Examples

```bash
# Write to default ~/.kube/config (will prompt before overwriting)
./create-kubeconfig-multiple-projects.sh

# Write to a custom location
./create-kubeconfig-multiple-projects.sh -f ~/.kube/stackit-config
```

---

## `delete-unused-volumes.sh`

Lists all STACKIT volumes in the currently configured project and deletes those with status `AVAILABLE`.

Set `DRY_RUN=1` at the top of the script to preview the deletions without executing them.

### Examples

```bash
# Make sure the right project is selected
stackit config set --project-id <project-id>

# Preview what would be deleted (edit DRY_RUN=1 in the script first)
./delete-unused-volumes.sh

# Actually delete unused volumes
./delete-unused-volumes.sh
```

---

## `ske-show-versions.sh`

Prints a tabular overview of SKE clusters across the project IDs configured in the `projectid` variable. For each nodepool it shows the Kubernetes version and the Flatcar machine image version, annotated with `supported` or the `expirationDate` for deprecated versions. Rows containing any deprecated version are highlighted in red.

The script enforces a minimum STACKIT CLI version (`0.59.0`).

Edit the `projectid="..."` variable (space-separated list) before running.

### Example

```bash
./ske-show-versions.sh
```

Sample output:

```
CLUSTER NAME         SKE VERSION                    NODEPOOL             FLATCAR VERSION                MACHINE TYPE    PROJECT ID
------------         -----------                    --------             ---------------                ------------    ----------
prod-cluster         1.30.4 (supported)             default              4081.2.0 (supported)           c1.4            xxxxxxxx-...
legacy-cluster       1.27.9 (exp. 2026-05-01)       default              3815.2.5 (exp. 2026-03-15)     c1.2            yyyyyyyy-...

Summary:
Total clusters: 2
```

---

## `smctl.sh`

Wrapper around the `vault` CLI to works with secrets in STACKIT Secrets Manager, think about it as `kubectl` but for secrets.

### Required environment variables

| Variable      | Description                                                    |
| ------------- | -------------------------------------------------------------- |
| `SM_USERNAME` | STACKIT Secret Manager username                                |
| `SM_PASSWORD` | STACKIT Secret Manager password                                |
| `SM_ID`       | KV secrets engine mount path (your secret manager instance ID) |

### Commands

| Command                          | Description                                                                                         |
| -------------------------------- | --------------------------------------------------------------------------------------------------- |
| `get <vault_path> <key>`         | Print the value of a single key.                                                                    |
| `get <vault_path> all`           | Print all keys as `key: value`.                                                                     |
| `get <vault_path> all-export`    | Print all keys as `export key=value` (suitable for `eval`/`source`).                                |
| `put <vault_path> <key> [value]` | Write/update a key. Reads from stdin if no value is given. Existing keys at the path are preserved. |
| `list`                           | List all secret paths under the mount.                                                              |
| `list <vault_path>`              | List all keys at a specific path.                                                                   |
| `help`                           | Show usage.                                                                                         |

### Examples

```bash
export SM_USERNAME='my-user'
export SM_PASSWORD='my-pass'
export SM_ID='sm-xxxxxxxx'

# List all paths
./smctl.sh list

# List keys at a path
./smctl.sh list postgresql

# Read a single value
./smctl.sh get postgresql db_password

# Dump everything at a path
./smctl.sh get postgresql all

# Load all secrets at a path into the current shell
eval "$(./smctl.sh get postgresql all-export)"

# Write a value directly
./smctl.sh put postgresql db_password 'super-secret'

# Write a value from a file (e.g. a full .env)
./smctl.sh put terraform secret-env < .env
```

---

## `list-project-resources.sh`

Iterates over the listable STACKIT services (DNS, Git, Load Balancers, SKE etc.) and prints a Markdown report containing one section per project with a table per resource type. Intended to be redirected into a `.md` file.

Project IDs are passed as arguments — at least one is required.

### Example

```bash
# Single project
./list-project-resources.sh xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx > project-inventory.md

# Multiple projects in one report
./list-project-resources.sh \
  xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy \
  > inventory.md
```

Sample output (excerpt):

```markdown
## Project: my-prod-project (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)

**SKE Clusters:**
|name|status|...|
|---|---|---|
|prod-cluster|HEALTHY|...|

**Public IPs:**
N/A

last update: Thu, 16-Apr-2026 14:42:11 CEST
```

---

## `vault-migrate.sh`

Migrates secrets between two KV v2-compatible secret backends, typically from a HashiCorp Vault instance into STACKIT Secrets Manager, or from one STACKIT Secrets Manager instance to another (both expose the Vault KV v2 API). Source authentication can be `userpass` or `ldap` (LDAP triggers an interactive password prompt); the target always uses `userpass`. Recursively walks all paths under the source mount unless `MIGRATE_PATHS` is set.

### Flags

- `-d, --dry-run` — show what would be migrated without writing.
- `-v, --verbose` — verbose progress output.
- `-s, --skip-existing` — skip secrets that already exist in the target.
- `--debug` — debug output (implies `--verbose`).
- `-h, --help` — show usage.

### Required environment variables

| Variable               | Description                                                       |
| ---------------------- | ----------------------------------------------------------------- |
| `SOURCE_VAULT_ADDR`    | Source Vault address (e.g. `https://vault.example.com`).          |
| `SOURCE_SM_ID`         | Source KV mount path.                                             |
| `SOURCE_AUTH_METHOD`   | `userpass` (default) or `ldap`.                                   |
| `SOURCE_SM_USERNAME`   | Source username — required when `SOURCE_AUTH_METHOD=userpass`.    |
| `SOURCE_SM_PASSWORD`   | Source password — required when `SOURCE_AUTH_METHOD=userpass`.    |
| `SOURCE_LDAP_USERNAME` | LDAP username — required when `SOURCE_AUTH_METHOD=ldap`.          |
| `TARGET_VAULT_ADDR`    | Target Vault address.                                             |
| `TARGET_SM_USERNAME`   | Target Vault username (always userpass).                          |
| `TARGET_SM_PASSWORD`   | Target Vault password.                                            |
| `TARGET_SM_ID`         | Target KV mount path.                                             |
| `MIGRATE_PATHS`        | Optional space-separated list of paths to migrate (default: all). |

### Examples

```bash
# Common target setup
export TARGET_VAULT_ADDR="https://prod.sm.eu01.stackit.cloud"
export TARGET_SM_USERNAME="target-user"
export TARGET_SM_PASSWORD='<your-target-password>'
export TARGET_SM_ID="sm-target-xxxx"

# 1) Dry run — migrate everything from a userpass source
export SOURCE_VAULT_ADDR="https://old.vault.example.com"
export SOURCE_SM_ID="sm-source-xxxx"
export SOURCE_AUTH_METHOD="userpass"
export SOURCE_SM_USERNAME="source-user"
export SOURCE_SM_PASSWORD='<your-source-password>'
./vault-migrate.sh --dry-run --verbose

# 2) Migrate from an LDAP source (will prompt for password)
export SOURCE_AUTH_METHOD="ldap"
export SOURCE_LDAP_USERNAME="myuser"
./vault-migrate.sh

# 3) Migrate only selected paths, skipping ones that already exist
export MIGRATE_PATHS="postgresql redis terraform"
./vault-migrate.sh --skip-existing --verbose
```
