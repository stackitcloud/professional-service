#!/bin/bash
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


# 1. Check for required dependencies
for cmd in stackit yq; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "Error: Required command '$cmd' is not installed or not in your PATH." >&2
    echo "Please install it before running this script." >&2
    exit 1
  fi
done

# Default path for kubeconfig
KUBECONFIG_PATH="$HOME/.kube/config"

# 2. Parse command line arguments for a custom filepath
while [[ "$#" -gt 0 ]]; do
  case $1 in
    -f|--filepath)
      KUBECONFIG_PATH="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [-f|--filepath <destination_path>]"
      exit 0
      ;;
    *)
      echo "Unknown parameter passed: $1"
      echo "Usage: $0 [-f|--filepath <destination_path>]"
      exit 1
      ;;
  esac
done

# 3. Safely check if the file exists and ask for confirmation
if [[ -f "$KUBECONFIG_PATH" ]]; then
  read -p "The file '$KUBECONFIG_PATH' already exists. Do you want to delete it before continuing? [y/N]: " confirm
  if [[ "$confirm" =~ ^[Yy]$ ]]; then
    rm "$KUBECONFIG_PATH"
    echo "Deleted '$KUBECONFIG_PATH'."
  else
    echo "Keeping the existing file. New configurations will be merged/appended."
  fi
fi

# Ensure the target directory exists just in case a custom path was provided
mkdir -p "$(dirname "$KUBECONFIG_PATH")"

# Export the KUBECONFIG environment variable so the STACKIT CLI targets the correct file
export KUBECONFIG="$KUBECONFIG_PATH"

stackit auth login

projects=(
  "xxx-xxx-xxx-xxx"
)

for project in "${projects[@]}"; do
  stackit config set --project-id "$project"

  clusters_yaml=$(stackit ske cluster list -o yaml)
  cluster_names=$(echo "$clusters_yaml" | yq e '.[] | .name' -)

  for cluster_name in $cluster_names; do
    echo "Creating kubeconfig for cluster: $cluster_name (saving to $KUBECONFIG_PATH)"
    stackit ske kubeconfig create --expiration 60d "$cluster_name" -y
  done
done
