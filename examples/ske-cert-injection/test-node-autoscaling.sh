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

NAMESPACE="default"
DEPLOYMENT="autoscaler-trigger"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-900}"
SLEEP_SECONDS="${SLEEP_SECONDS:-10}"

cleanup() {
  kubectl delete deployment "${DEPLOYMENT}" \
    --namespace "${NAMESPACE}" \
    --ignore-not-found
}
trap cleanup EXIT

ready_node_count() {
  ready_node_names \
    | wc -l \
    | tr -d ' '
}

ready_node_names() {
  kubectl get nodes --no-headers \
    | awk '$2 == "Ready" { print $1 }' \
    | sort
}

BASELINE_COUNT="$(ready_node_count)"
BASELINE_NODES="$(mktemp)"
CURRENT_NODES="$(mktemp)"

rm_temp_files() {
  rm -f "${BASELINE_NODES}" "${CURRENT_NODES}"
}
trap 'cleanup; rm_temp_files' EXIT

ready_node_names >"${BASELINE_NODES}"

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${DEPLOYMENT}
  namespace: ${NAMESPACE}
  labels:
    app: ${DEPLOYMENT}
spec:
  replicas: 5
  selector:
    matchLabels:
      app: ${DEPLOYMENT}
  template:
    metadata:
      labels:
        app: ${DEPLOYMENT}
    spec:
      containers:
        - name: pause
          image: registry.k8s.io/pause:3.10
          imagePullPolicy: IfNotPresent
          resources:
            requests:
              cpu: "1"
              memory: "1Gi"
EOF

deadline=$((SECONDS + TIMEOUT_SECONDS))
NEW_NODE_NAME=""

while (( SECONDS < deadline )); do
  CURRENT_COUNT="$(ready_node_count)"

  if (( CURRENT_COUNT > BASELINE_COUNT )); then
    ready_node_names >"${CURRENT_NODES}"
    NEW_NODE_NAME="$(comm -13 "${BASELINE_NODES}" "${CURRENT_NODES}" | head -n 1)"

    if [[ -n "${NEW_NODE_NAME}" ]]; then
      break
    fi
  fi

  sleep "${SLEEP_SECONDS}"
done

if [[ -z "${NEW_NODE_NAME}" ]]; then
  echo "ERROR: timed out waiting for a new Ready node." >&2
  exit 1
fi

deadline=$((SECONDS + TIMEOUT_SECONDS))
INJECTOR_POD=""

while (( SECONDS < deadline )); do
  INJECTOR_POD="$(kubectl get pods \
    --namespace kube-system \
    --selector app=node-ca-injector \
    --field-selector "spec.nodeName=${NEW_NODE_NAME}" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"

  if [[ -n "${INJECTOR_POD}" ]]; then
    PHASE="$(kubectl get pod "${INJECTOR_POD}" \
      --namespace kube-system \
      -o jsonpath='{.status.phase}')"

    if [[ "${PHASE}" == "Running" ]]; then
      echo "Node autoscaling test passed. New node: ${NEW_NODE_NAME}. Injector pod: ${INJECTOR_POD}."
      exit 0
    fi
  fi

  sleep "${SLEEP_SECONDS}"
done

echo "ERROR: timed out waiting for node-ca-injector to run on new node ${NEW_NODE_NAME}." >&2
exit 1
