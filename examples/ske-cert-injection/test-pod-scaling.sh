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

NAMESPACE="dynamic-ca-test"
DEPLOYMENT="dynamic-ca-client"
TARGET_URL="https://dummy-server.default.svc.cluster.local"
TIMEOUT="${TIMEOUT:-300s}"

cleanup() {
  kubectl delete namespace "${NAMESPACE}" --ignore-not-found
}
trap cleanup EXIT

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo "Waiting for trust-manager to create trusted-ca-bundle in namespace ${NAMESPACE}..."
end_time=$((SECONDS + 300))
until kubectl get configmap trusted-ca-bundle --namespace "${NAMESPACE}" >/dev/null 2>&1; do
  if (( SECONDS >= end_time )); then
    echo "ERROR: timed out waiting for trusted-ca-bundle ConfigMap in namespace ${NAMESPACE}." >&2
    exit 1
  fi
  sleep 5
done

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
        - name: alpine
          image: alpine:latest
          imagePullPolicy: IfNotPresent
          command:
            - /bin/sh
            - -c
            - sleep 3600
          volumeMounts:
            - name: trusted-ca-bundle
              mountPath: /etc/ssl/certs/ca-certificates.crt
              subPath: ca-certificates.crt
              readOnly: true
      volumes:
        - name: trusted-ca-bundle
          configMap:
            name: trusted-ca-bundle
EOF

kubectl rollout status deployment/"${DEPLOYMENT}" \
  --namespace "${NAMESPACE}" \
  --timeout="${TIMEOUT}"

kubectl wait \
  --for=condition=Ready \
  pods \
  --selector app="${DEPLOYMENT}" \
  --namespace "${NAMESPACE}" \
  --timeout="${TIMEOUT}"

POD="$(kubectl get pods \
  --namespace "${NAMESPACE}" \
  --selector app="${DEPLOYMENT}" \
  --field-selector=status.phase=Running \
  -o jsonpath='{.items[0].metadata.name}')"

if [[ -z "${POD}" ]]; then
  echo "ERROR: no running pod found for deployment ${DEPLOYMENT}." >&2
  exit 1
fi

kubectl exec "${POD}" \
  --namespace "${NAMESPACE}" \
  -- wget -q -O- "${TARGET_URL}"

echo
echo "Scaling test passed."
