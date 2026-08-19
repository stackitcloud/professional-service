#!/usr/bin/env bash
# Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Vault Secret Migration Script - Migrate secrets between Vault instances using v2 API
# Usage: vault-migrate.sh [options]

set -euo pipefail

# Default values
DRY_RUN=false
VERBOSE=false
SKIP_EXISTING=false
DEBUG=false

# Source Vault configuration
: "${SOURCE_VAULT_ADDR:?Environment variable SOURCE_VAULT_ADDR is not set}"
: "${SOURCE_SM_ID:?Environment variable SOURCE_SM_ID is not set (KV mount path)}"

# Source authentication method (userpass or ldap)
SOURCE_AUTH_METHOD="${SOURCE_AUTH_METHOD:-userpass}"

# Validate source authentication variables based on method
if [ "${SOURCE_AUTH_METHOD}" = "userpass" ]; then
  : "${SOURCE_SM_USERNAME:?Environment variable SOURCE_SM_USERNAME is not set (required for userpass auth)}"
  : "${SOURCE_SM_PASSWORD:?Environment variable SOURCE_SM_PASSWORD is not set (required for userpass auth)}"
elif [ "${SOURCE_AUTH_METHOD}" = "ldap" ]; then
  : "${SOURCE_LDAP_USERNAME:?Environment variable SOURCE_LDAP_USERNAME is not set (required for ldap auth)}"
  # Note: LDAP password is NOT required as env var - will use interactive login
else
  echo "Error: SOURCE_AUTH_METHOD must be 'userpass' or 'ldap' (got: ${SOURCE_AUTH_METHOD})"
  exit 1
fi

# Target Vault configuration
: "${TARGET_VAULT_ADDR:?Environment variable TARGET_VAULT_ADDR is not set}"
: "${TARGET_SM_USERNAME:?Environment variable TARGET_SM_USERNAME is not set}"
: "${TARGET_SM_PASSWORD:?Environment variable TARGET_SM_PASSWORD is not set}"
: "${TARGET_SM_ID:?Environment variable TARGET_SM_ID is not set (KV mount path)}"

# Optional: specify paths to migrate (if not set, migrate all)
MIGRATE_PATHS="${MIGRATE_PATHS:-}"

# Check for required dependencies
for cmd in vault jq; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: Required command '$cmd' not found in PATH"
    exit 1
  fi
done

# Function to display usage
usage() {
  cat <<EOF
Usage: $0 [options]

Migrate secrets from one Vault instance to another using Vault v2 KV API.

Options:
  -d, --dry-run          Show what would be migrated without making changes
  -v, --verbose          Enable verbose output
  -s, --skip-existing    Skip secrets that already exist in target
  --debug                Enable debug output (shows all vault commands)
  -h, --help             Show this help message

Environment variables required:

Source Vault:
  SOURCE_VAULT_ADDR      Source Vault address (e.g., https://vault.example.com)
  SOURCE_SM_ID           Source KV secrets engine mount path
  SOURCE_AUTH_METHOD     Authentication method: "userpass" or "ldap" (default: userpass)

  For userpass authentication:
    SOURCE_SM_USERNAME   Source Vault username
    SOURCE_SM_PASSWORD   Source Vault password

  For LDAP authentication (interactive password prompt):
    SOURCE_LDAP_USERNAME LDAP username

Target Vault (always uses userpass):
  TARGET_VAULT_ADDR      Target Vault address
  TARGET_SM_USERNAME     Target Vault username for authentication
  TARGET_SM_PASSWORD     Target Vault password for authentication
  TARGET_SM_ID           Target KV secrets engine mount path

Optional:
  MIGRATE_PATHS          Space-separated list of paths to migrate (default: all)
                         Example: export MIGRATE_PATHS="postgresql redis terraform"

Examples:
  # Migrate all secrets (source uses userpass)
  $ export SOURCE_AUTH_METHOD="userpass"
  $ export SOURCE_SM_USERNAME="user" SOURCE_SM_PASSWORD="pass"
  $ vault-migrate.sh

  # Migrate using LDAP for source authentication (will prompt for password)
  $ export SOURCE_AUTH_METHOD="ldap"
  $ export SOURCE_LDAP_USERNAME="myuser"
  $ vault-migrate.sh --dry-run --verbose

  # Migrate specific paths only with LDAP
  $ export SOURCE_AUTH_METHOD="ldap"
  $ export SOURCE_LDAP_USERNAME="myuser"
  $ export MIGRATE_PATHS="postgresql redis"
  $ vault-migrate.sh

  # Skip secrets that already exist in target
  $ vault-migrate.sh --skip-existing --verbose

EOF
  exit 1
}

# Function to authenticate to source Vault
authenticate_source() {
  if [ -z "${SOURCE_VAULT_TOKEN:-}" ]; then
    if [ "${SOURCE_AUTH_METHOD}" = "userpass" ]; then
      log_verbose "Authenticating to source Vault at ${SOURCE_VAULT_ADDR} (method: userpass, user: ${SOURCE_SM_USERNAME})..."
      SOURCE_VAULT_TOKEN=$(vault login --address "${SOURCE_VAULT_ADDR}" -no-store -format=json \
        --method=userpass username="${SOURCE_SM_USERNAME}" password="${SOURCE_SM_PASSWORD}" 2>/dev/null | jq -r .auth.client_token)
    elif [ "${SOURCE_AUTH_METHOD}" = "ldap" ]; then
      log_info "Authenticating to source Vault at ${SOURCE_VAULT_ADDR} (method: ldap, user: ${SOURCE_LDAP_USERNAME})..."

      # Interactive LDAP login - vault will prompt for password
      if ! vault login -address="${SOURCE_VAULT_ADDR}" -method=ldap username="${SOURCE_LDAP_USERNAME}" >/dev/null 2>&1; then
        echo "Error: Failed to authenticate to source Vault using LDAP"
        exit 1
      fi

      # Read token from vault token file
      if [ -f "${HOME}/.vault-token" ]; then
        SOURCE_VAULT_TOKEN=$(cat "${HOME}/.vault-token")
      else
        echo "Error: Vault token file not found at ${HOME}/.vault-token"
        exit 1
      fi
    fi

    if [ -z "${SOURCE_VAULT_TOKEN}" ] || [ "${SOURCE_VAULT_TOKEN}" = "null" ]; then
      echo "Error: Failed to authenticate to source Vault using ${SOURCE_AUTH_METHOD}"
      exit 1
    fi
    log_verbose "✓ Source authentication successful"
  fi
}

# Function to authenticate to target Vault
authenticate_target() {
  if [ -z "${TARGET_VAULT_TOKEN:-}" ]; then
    log_verbose "Authenticating to target Vault at ${TARGET_VAULT_ADDR}..."
    TARGET_VAULT_TOKEN=$(vault login --address "${TARGET_VAULT_ADDR}" -no-store -format=json \
      --method=userpass username="${TARGET_SM_USERNAME}" password="${TARGET_SM_PASSWORD}" 2>/dev/null | jq -r .auth.client_token)

    if [ -z "${TARGET_VAULT_TOKEN}" ] || [ "${TARGET_VAULT_TOKEN}" = "null" ]; then
      echo "Error: Failed to authenticate to target Vault"
      exit 1
    fi
    log_verbose "✓ Target authentication successful"
  fi
}

# Logging functions
log_verbose() {
  if [ "${VERBOSE}" = true ]; then
    echo "$@" >&2
  fi
}

log_debug() {
  if [ "${DEBUG}" = true ]; then
    echo "[DEBUG] $@" >&2
  fi
}

log_info() {
  echo "$@" >&2
}

log_error() {
  echo "ERROR: $@" >&2
}

# Function to list paths at a given location
list_paths_at() {
  local base_path="$1"
  local error_output
  error_output=$(mktemp)

  local paths
  # Try JSON format first
  paths=$(VAULT_ADDR="${SOURCE_VAULT_ADDR}" VAULT_TOKEN="${SOURCE_VAULT_TOKEN}" \
    vault kv list -mount="${SOURCE_SM_ID}" "${base_path}" -format=json 2>"${error_output}" | jq -r '.[]' 2>/dev/null || echo "")

  # If JSON failed, try table format
  if [ -z "${paths}" ]; then
    paths=$(VAULT_ADDR="${SOURCE_VAULT_ADDR}" VAULT_TOKEN="${SOURCE_VAULT_TOKEN}" \
      vault kv list -mount="${SOURCE_SM_ID}" "${base_path}" 2>"${error_output}" | grep -v "^Keys$" | grep -v "^----$" | grep -v "^$" || echo "")
  fi

  rm -f "${error_output}"
  echo "${paths}"
}

# Function to recursively get all secret paths from source Vault
get_all_paths_recursive() {
  local prefix="$1"
  local paths

  paths=$(list_paths_at "${prefix}")

  for item in ${paths}; do
    if [[ "${item}" == */ ]]; then
      # It's a folder, recurse into it
      log_debug "Found folder: ${prefix}${item}"
      get_all_paths_recursive "${prefix}${item}"
    else
      # It's a secret, add it to the list
      echo "${prefix}${item}"
    fi
  done
}

# Function to get all paths from source Vault
get_all_paths() {
  authenticate_source

  log_verbose "Fetching all secret paths from source Vault..."

  local cmd="VAULT_ADDR=\"${SOURCE_VAULT_ADDR}\" VAULT_TOKEN=\"${SOURCE_VAULT_TOKEN}\" vault kv list -mount=\"${SOURCE_SM_ID}\""
  log_debug "Command: ${cmd}"

  # Get all paths recursively starting from root
  local all_secrets
  all_secrets=$(get_all_paths_recursive "")

  if [ -z "${all_secrets}" ]; then
    log_error "No secrets found in source Vault"
    log_error "Mount path: ${SOURCE_SM_ID}"
    exit 1
  fi

  log_debug "Found secrets: ${all_secrets}"
  echo "${all_secrets}"
}

# Function to get secret from source
get_source_secret() {
  local path="$1"

  authenticate_source

  local cmd="VAULT_ADDR=\"${SOURCE_VAULT_ADDR}\" VAULT_TOKEN=\"<hidden>\" vault kv get -mount=\"${SOURCE_SM_ID}\" -format=json \"${path}\""
  log_debug "Command: ${cmd}"

  local error_output
  error_output=$(mktemp)

  local result
  result=$(VAULT_ADDR="${SOURCE_VAULT_ADDR}" VAULT_TOKEN="${SOURCE_VAULT_TOKEN}" \
    vault kv get -mount="${SOURCE_SM_ID}" -format=json "${path}" 2>"${error_output}" | jq -r '.data.data')

  local exit_code=$?

  if [ ${exit_code} -ne 0 ] || [ -z "${result}" ] || [ "${result}" = "null" ]; then
    log_debug "Failed to read secret at path: ${path}"
    if [ -s "${error_output}" ]; then
      log_debug "Vault error output:"
      cat "${error_output}" >&2
    fi
    rm -f "${error_output}"
    echo ""
    return 1
  fi

  rm -f "${error_output}"
  echo "${result}"
}

# Function to check if secret exists in target
target_secret_exists() {
  local path="$1"

  authenticate_target

  VAULT_ADDR="${TARGET_VAULT_ADDR}" VAULT_TOKEN="${TARGET_VAULT_TOKEN}" \
    vault kv get -mount="${TARGET_SM_ID}" -format=json "${path}" 2>/dev/null >/dev/null

  return $?
}

# Function to put secret to target
put_target_secret() {
  local path="$1"
  local secret_data="$2"

  authenticate_target

  # Convert JSON object to key=value arguments
  local put_args=()
  while IFS= read -r key; do
    local value=$(echo "${secret_data}" | jq -r --arg k "$key" '.[$k]')
    put_args+=("${key}=${value}")
  done < <(echo "${secret_data}" | jq -r 'keys[]')

  if [ ${#put_args[@]} -eq 0 ]; then
    log_error "No key-value pairs found in secret data for path '${path}'"
    return 1
  fi

  VAULT_ADDR="${TARGET_VAULT_ADDR}" VAULT_TOKEN="${TARGET_VAULT_TOKEN}" \
    vault kv put -mount="${TARGET_SM_ID}" "${path}" "${put_args[@]}" >/dev/null 2>&1
}

# Function to migrate a single secret path
migrate_secret() {
  local path="$1"
  local migrated_count=0
  local skipped_count=0
  local failed_count=0

  log_info "Processing: ${path}"

  # Check if secret exists in target and skip if requested
  if [ "${SKIP_EXISTING}" = true ]; then
    if target_secret_exists "${path}"; then
      log_info "  ⊘ Skipped (already exists in target)"
      echo "0:1:0"
      return 0
    fi
  fi

  # Get secret from source
  local secret_data
  secret_data=$(get_source_secret "${path}")

  if [ -z "${secret_data}" ] || [ "${secret_data}" = "null" ]; then
    log_info "  ⊘ Skipped (empty or deleted secret)"
    log_verbose "     Path: ${path}"
    echo "0:1:0"
    return 0
  fi

  # Count keys
  local key_count
  key_count=$(echo "${secret_data}" | jq 'keys | length')
  log_verbose "  Found ${key_count} keys"

  # Dry run mode
  if [ "${DRY_RUN}" = true ]; then
    log_info "  → Would migrate ${key_count} keys (dry-run)"
    if [ "${VERBOSE}" = true ]; then
      echo "${secret_data}" | jq -r 'to_entries[] | "    - \(.key)"' >&2
    fi
    echo "0:0:0"
    return 0
  fi

  # Put secret to target
  if put_target_secret "${path}" "${secret_data}"; then
    log_info "  ✓ Migrated ${key_count} keys"
    echo "1:0:0"
  else
    log_error "  ✗ Failed to write to target"
    echo "0:0:1"
  fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -d|--dry-run)
      DRY_RUN=true
      shift
      ;;
    -v|--verbose)
      VERBOSE=true
      shift
      ;;
    -s|--skip-existing)
      SKIP_EXISTING=true
      shift
      ;;
    --debug)
      DEBUG=true
      VERBOSE=true  # Debug implies verbose
      shift
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Error: Unknown option: $1"
      echo
      usage
      ;;
  esac
done

# Main migration logic
main() {
  log_info "=== Vault Secret Migration ==="
  log_info "Source: ${SOURCE_VAULT_ADDR} (mount: ${SOURCE_SM_ID}, auth: ${SOURCE_AUTH_METHOD})"
  log_info "Target: ${TARGET_VAULT_ADDR} (mount: ${TARGET_SM_ID}, auth: userpass)"

  if [ "${DRY_RUN}" = true ]; then
    log_info "Mode: DRY RUN (no changes will be made)"
  fi

  log_info ""

  # Authenticate to both vaults
  authenticate_source
  authenticate_target

  # Determine which paths to migrate
  local paths_to_migrate
  if [ -n "${MIGRATE_PATHS}" ]; then
    log_info "Migrating specified paths: ${MIGRATE_PATHS}"
    paths_to_migrate="${MIGRATE_PATHS}"
  else
    log_info "Migrating all paths from source Vault..."
    paths_to_migrate=$(get_all_paths)
  fi

  log_info ""

  # Statistics
  local total_migrated=0
  local total_skipped=0
  local total_failed=0
  local total_paths=0

  # Migrate each path
  for path in ${paths_to_migrate}; do
    ((total_paths++)) || true

    result=$(migrate_secret "${path}")
    IFS=':' read -r migrated skipped failed <<< "${result}"

    ((total_migrated+=migrated)) || true
    ((total_skipped+=skipped)) || true
    ((total_failed+=failed)) || true
  done

  # Print summary
  log_info ""
  log_info "=== Migration Summary ==="
  log_info "Total paths processed: ${total_paths}"

  if [ "${DRY_RUN}" = false ]; then
    log_info "Successfully migrated: ${total_migrated}"
    if [ "${SKIP_EXISTING}" = true ]; then
      log_info "Skipped (existing):    ${total_skipped}"
    fi
    log_info "Failed:                ${total_failed}"

    if [ ${total_failed} -gt 0 ]; then
      log_error "Migration completed with errors"
      exit 1
    else
      log_info ""
      log_info "✓ Migration completed successfully!"
    fi
  else
    log_info "✓ Dry run completed"
  fi
}

# Run main function
main
