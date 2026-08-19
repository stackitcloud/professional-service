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

# Unified Secret Manager utility for STACKIT Secret Manager (Vault v2)
# Usage: smkey.sh <command> [args]

set -euo pipefail

# Check for required dependencies
for cmd in vault jq; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "Error: Required command '$cmd' is not installed or not in your PATH." >&2
    echo "Please install it before running this script." >&2
    exit 1
  fi
done

export VAULT_ADDR="https://prod.sm.eu01.stackit.cloud"

# Ensure required env variables are set
: "${SM_USERNAME:?Environment variable SM_USERNAME is not set}"
: "${SM_PASSWORD:?Environment variable SM_PASSWORD is not set}"
: "${SM_ID:?Environment variable SM_ID is not set}"

# Function to authenticate and get Vault token
authenticate() {
  if [ -z "${VAULT_TOKEN:-}" ]; then
    export VAULT_TOKEN=$(vault login --address "${VAULT_ADDR}" -no-store -format=json \
      --method=userpass username="${SM_USERNAME}" password="${SM_PASSWORD}" 2>/dev/null | jq -r .auth.client_token)
  fi
}

# Function to display usage
usage() {
  cat <<EOF
Usage: $0 <command> [args]

Commands:
  get <vault_path> <key>          Get a specific secret value
                                  Example: $0 get postgresql db_password

  get <vault_path> all            Get all key-value pairs in "key: value" format
                                  Example: $0 get postgresql all

  get <vault_path> all-export     Get all key-value pairs in "export key=value" format
                                  Example: $0 get postgresql all-export

  put <vault_path> <key> [value]  Put a secret value (from argument or stdin)
                                  Example: $0 put postgresql db_password myvalue123
                                  Example: $0 put terraform secret-env < .env

  list [vault_path]               List all available vault paths (no arg)
                                  or list all keys in a specific vault path
                                  Example: $0 list
                                  Example: $0 list postgresql

  help                            Show this help message

Environment variables required:
  SM_USERNAME    STACKIT Secret Manager username
  SM_PASSWORD    STACKIT Secret Manager password
  SM_ID          KV secrets engine mount path
EOF
  exit 1
}

# Command: get
cmd_get() {
  if [ $# -ne 2 ]; then
    echo "Error: 'get' requires <vault_path> and <key> arguments"
    echo "Usage: $0 get <vault_path> <key>"
    echo "       $0 get <vault_path> all          # Get all keys in 'key: value' format"
    echo "       $0 get <vault_path> all-export   # Get all keys in 'export key=value' format"
    exit 1
  fi

  local vault_path="$1"
  local secret_key="$2"

  authenticate

  # Handle special "all" and "all-export" keys
  if [ "${secret_key}" = "all" ]; then
    # Get all key-value pairs in "key: value" format
    local secret_data=$(vault kv get -mount="${SM_ID}" -format=json "${vault_path}" 2>/dev/null)

    if [ $? -ne 0 ] || [ -z "${secret_data}" ]; then
      echo "Error: No secret found at path '${vault_path}'"
      exit 1
    fi

    # Output in simple "key: value" format
    echo "${secret_data}" | jq -r '.data.data | to_entries[] | "\(.key): \(.value)"'
    return
  fi

  if [ "${secret_key}" = "all-export" ]; then
    # Get all key-value pairs in "export key=value" format
    local secret_data=$(vault kv get -mount="${SM_ID}" -format=json "${vault_path}" 2>/dev/null)

    if [ $? -ne 0 ] || [ -z "${secret_data}" ]; then
      echo "Error: No secret found at path '${vault_path}'"
      exit 1
    fi

    # Output in "export key=value" format
    echo "${secret_data}" | jq -r '.data.data | to_entries[] | "export \(.key)=\(.value)"'
    return
  fi

  # Standard single key retrieval
  vault kv get -mount="${SM_ID}" -field="${secret_key}" "${vault_path}"
}

# Command: put
cmd_put() {
  if [ $# -lt 2 ] || [ $# -gt 3 ]; then
    echo "Error: 'put' requires <vault_path> and <key> arguments, with optional <value>"
    echo "Usage: $0 put <vault_path> <key> <value>       # Provide value directly"
    echo "       $0 put <vault_path> <key> < input_file  # Read value from file"
    exit 1
  fi

  local vault_path="$1"
  local secret_key="$2"

  authenticate

  # Get current secret data (if exists)
  local current_data=$(vault kv get -mount="${SM_ID}" -format=json "${vault_path}" 2>/dev/null | jq -r '.data.data' || echo "{}")

  # Read new value from argument or stdin
  local new_value
  if [ $# -eq 3 ]; then
    # Value provided as argument
    new_value="$3"
  else
    # Read value from stdin
    new_value=$(cat)
  fi

  # Merge existing keys with new key/value
  local updated_data=$(echo "${current_data}" | jq --arg k "${secret_key}" --arg v "${new_value}" '. + {($k): $v}')

  # Build arguments for vault kv put
  local put_args=()
  while IFS= read -r key; do
    local value=$(echo "${updated_data}" | jq -r --arg k "$key" '.[$k]')
    put_args+=("${key}=${value}")
  done < <(echo "${updated_data}" | jq -r 'keys[]')

  # Write merged data back to Vault
  vault kv put -mount="${SM_ID}" "${vault_path}" "${put_args[@]}"
}

# Command: list
cmd_list() {
  authenticate

  # If no path provided, list all vault paths
  if [ $# -eq 0 ]; then
    echo "Available vault paths:"
    vault kv list -mount="${SM_ID}" -format=json | jq -r '.[]' 2>/dev/null || {
      echo "Error: Unable to list vault paths or no paths found"
      exit 1
    }
    return
  fi

  # If path provided, list all keys in that path
  if [ $# -ne 1 ]; then
    echo "Error: 'list' requires zero or one argument"
    echo "Usage: $0 list [vault_path]"
    exit 1
  fi

  local vault_path="$1"

  # Get the secret and list all keys
  local secret_data=$(vault kv get -mount="${SM_ID}" -format=json "${vault_path}" 2>/dev/null)

  if [ $? -ne 0 ] || [ -z "${secret_data}" ]; then
    echo "Error: No secret found at path '${vault_path}'"
    exit 1
  fi

  echo "Keys in ${vault_path}:"
  echo "${secret_data}" | jq -r '.data.data | keys[]'
}

# Main command dispatcher
if [ $# -eq 0 ]; then
  usage
fi

COMMAND="$1"
shift

case "${COMMAND}" in
  get)
    cmd_get "$@"
    ;;
  put)
    cmd_put "$@"
    ;;
  list)
    cmd_list "$@"
    ;;
  help|--help|-h)
    usage
    ;;
  *)
    echo "Error: Unknown command '${COMMAND}'"
    echo
    usage
    ;;
esac
