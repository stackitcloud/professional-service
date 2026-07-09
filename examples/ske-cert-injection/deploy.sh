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

# Rolls out the full Kubernetes Custom CA Injection PoC in dependency order.
#
# The order matters:
#   1. generate the fake CA and server certificate, create dummy-tls, and render
#      the node-level CA ConfigMap manifest
#   2. deploy the HTTPS dummy server that uses the generated dummy-tls Secret
#   3. deploy the node-level ConfigMap and privileged DaemonSet injector
#   4. install cert-manager and trust-manager with kube-system as trust namespace
#   5. create the trust-manager Bundle and wait for it to sync to ConfigMaps
#   6. create the two test Pods used by verify.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TIMEOUT="${TIMEOUT:-300s}"

command -v kubectl >/dev/null 2>&1 || {
  echo "ERROR: kubectl is required to deploy this PoC." >&2
  exit 1
}

echo "Phase 00a: generating certificates and Kubernetes TLS Secret..."
"${SCRIPT_DIR}/00-simulation/generate-certs.sh"

echo "Phase 00b: deploying dummy HTTPS server..."
kubectl apply -f "${SCRIPT_DIR}/00-simulation/dummy-https-server.yaml"
kubectl wait \
  --for=condition=Ready \
  pod/dummy-server \
  --namespace default \
  --timeout="${TIMEOUT}"

echo "Phase 01: deploying node-level CA ConfigMap and injector DaemonSet..."
kubectl apply -f "${SCRIPT_DIR}/01-node-level/configmap-ca.yaml"
kubectl apply -f "${SCRIPT_DIR}/01-node-level/daemonset-injector.yaml"
kubectl rollout status daemonset/node-ca-injector \
  --namespace kube-system \
  --timeout="${TIMEOUT}"

echo "Phase 02: installing cert-manager and trust-manager..."
"${SCRIPT_DIR}/02-pod-level/install-trust-manager.sh"

echo "Phase 02: applying trust-manager Bundle..."
kubectl apply -f "${SCRIPT_DIR}/02-pod-level/bundle.yaml"

echo "Waiting for trusted-ca-bundle Bundle to report Synced=True..."
kubectl wait \
  --for=condition=Synced=True \
  bundle/trusted-ca-bundle \
  --timeout="${TIMEOUT}"

echo "Waiting for trusted-ca-bundle ConfigMap to appear in default namespace..."
kubectl wait \
  --for=create \
  configmap/trusted-ca-bundle \
  --namespace default \
  --timeout="${TIMEOUT}"

echo "Phase 03: deploying test clients..."
kubectl apply -f "${SCRIPT_DIR}/03-test-clients/client-fail.yaml"
kubectl apply -f "${SCRIPT_DIR}/03-test-clients/client-success.yaml"

echo "Waiting for test clients to report Ready..."
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

echo "Deployment complete. Run ${SCRIPT_DIR}/verify.sh to execute the E2E checks."
