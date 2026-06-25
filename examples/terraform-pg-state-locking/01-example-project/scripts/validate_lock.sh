#!/bin/bash
set -u

echo "[INFO] Initiating background 'terraform apply' to acquire the state lock..."
# Redirecting output to avoid console clutter during the concurrent test
terraform apply -auto-approve > apply_bg.log 2>&1 &
APPLY_PID=$!

echo "[INFO] Attempting concurrent 'terraform plan'..."
echo "[INFO] ------------------------------------------------------------------"

# Disable exit-on-error to capture the expected failure code
set +e
terraform plan
PLAN_EXIT_CODE=$?
set -e

echo "[INFO] ------------------------------------------------------------------"

if [ $PLAN_EXIT_CODE -ne 0 ]; then
  echo "[SUCCESS] Concurrent operation rejected. State locking is active and functional."
else
  echo "[ERROR] Concurrent operation succeeded. State locking failed or is misconfigured."
fi

echo "[INFO] Waiting for the background 'terraform apply' process to terminate..."
wait $APPLY_PID

echo "[INFO] Evaluation complete. Cleaning up temporary logs..."
rm -f apply_bg.log
