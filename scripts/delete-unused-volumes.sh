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

# Set to 1 to only print the volumes that would be deleted (no actual deletion)
DRY_RUN=0

echo "Fetching volumes..."

# Extract only IDs for deletion
volume_ids=$(stackit volume list -o yaml | yq -r '.[] | select(.status == "AVAILABLE") | .id')

echo ""
for id in $volume_ids; do
  echo "Deleting volume ID: $id"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "[Dry run] stackit volume delete $id"
  else
    stackit volume delete "$id" -y
    if [[ $? -ne 0 ]]; then
      echo "❌ Failed to delete volume $id"
    else
      echo "✅ Deleted volume $id"
    fi
  fi
done

echo "Done."
