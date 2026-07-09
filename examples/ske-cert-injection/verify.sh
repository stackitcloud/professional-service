#!/usr/bin/env bash
#
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

set -euo pipefail

# Verifies the PoC end to end:
#   - the node-level injector is fully ready
#   - the trust-manager Bundle is synced
#   - both test Pods are Ready before any exec calls
#   - the unmounted client fails TLS validation
#   - the client with the mounted CA bundle succeeds

TIMEOUT="${TIMEOUT:-300s}"
TARGET_URL="${TARGET_URL:-https://dummy-server.default.svc.cluster.local}"

command -v kubectl >/dev/null 2>&1 || {
  echo "ERROR: kubectl is required to verify this PoC." >&2
  exit 1
}

echo "Node Test: verifying node-ca-injector DaemonSet READY equals DESIRED..."
DESIRED="$(kubectl get daemonset node-ca-injector \
  --namespace kube-system \
  -o jsonpath='{.status.desiredNumberScheduled}')"
READY="$(kubectl get daemonset node-ca-injector \
  --namespace kube-system \
  -o jsonpath='{.status.numberReady}')"

if [[ "${DESIRED}" != "${READY}" ]]; then
  echo "FAIL: node-ca-injector READY=${READY}, DESIRED=${DESIRED}."
  exit 1
fi
echo "PASS: node-ca-injector READY=${READY}, DESIRED=${DESIRED}."

echo "Trust-Manager Test: verifying trusted-ca-bundle Bundle reports Synced=True..."
SYNCED_STATUS="$(kubectl get bundle trusted-ca-bundle \
  -o jsonpath='{.status.conditions[?(@.type=="Synced")].status}')"

if [[ "${SYNCED_STATUS}" != "True" ]]; then
  echo "FAIL: trusted-ca-bundle Synced status is '${SYNCED_STATUS:-unset}', expected True."
  exit 1
fi
echo "PASS: trusted-ca-bundle reports Synced=True."

echo "Pod Readiness Wait: waiting for client-fail and client-success to become Ready..."
kubectl wait \
  --for=condition=Ready \
  pod/client-fail \
  --namespace default \
  --timeout="${TIMEOUT}"
kubectl wait \
  --for=condition=Ready \
  pod/client-success \
  --namespace default \
  --timeout="${TIMEOUT}"
echo "PASS: both test Pods are Ready."

echo "Negative E2E Test: client-fail should reject ${TARGET_URL} because it lacks the custom CA..."
set +e
kubectl exec client-fail \
  --namespace default \
  -- wget -q -O- "${TARGET_URL}"
NEGATIVE_EXIT_CODE=$?
set -e

if [[ "${NEGATIVE_EXIT_CODE}" -eq 0 ]]; then
  echo "FAIL: client-fail unexpectedly trusted the dummy HTTPS server."
  exit 1
fi
echo "PASS: client-fail rejected the dummy HTTPS server as expected."

echo "Positive E2E Test: client-success should trust ${TARGET_URL} through the mounted CA bundle..."
kubectl exec client-success \
  --namespace default \
  -- wget -q -O- "${TARGET_URL}"
echo
echo "PASS: client-success reached the dummy HTTPS server successfully."

echo "All verification checks passed."
