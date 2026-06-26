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
