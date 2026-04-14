#!/bin/bash

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
