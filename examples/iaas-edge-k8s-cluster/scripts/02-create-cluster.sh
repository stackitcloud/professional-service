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

set -euo pipefail

KUBECONFIG="${KUBECONFIG:?}"
CLUSTER_NAME="${CLUSTER_NAME:?}"
TALOS_VERSION="${TALOS_VERSION:?}"
KUBERNETES_VERSION="${KUBERNETES_VERSION:?}"
OUTPUT_DIR="${OUTPUT_DIR:?}"
# Comma-separated STACKIT IaaS server UUIDs. EdgeHost names equal server UUIDs,
# so this mapping is used to assign controlplane/worker roles reliably regardless
# of EdgeHost registration order.
CP_SERVER_IDS="${CP_SERVER_IDS:?}"
WORKER_SERVER_IDS="${WORKER_SERVER_IDS:?}"

KUBECTL="kubectl --kubeconfig=${KUBECONFIG}"
mkdir -p "${OUTPUT_DIR}"

IFS=',' read -ra CP_IDS    <<< "${CP_SERVER_IDS}"
IFS=',' read -ra WORKER_IDS <<< "${WORKER_SERVER_IDS}"
EXPECTED=$(( ${#CP_IDS[@]} + ${#WORKER_IDS[@]} ))

# Returns 0 if $1 is found in the comma-separated list $2.
in_list() { echo ",${2}," | grep -qF ",${1},"; }

# --- Wait for all EdgeHosts -------------------------------------------------

>&2 echo "==> Waiting for ${EXPECTED} EdgeHosts (${#CP_IDS[@]} CP + ${#WORKER_IDS[@]} worker, timeout: 20 min)..."
TIMEOUT=1200 INTERVAL=15 ELAPSED=0

while true; do
  FOUND=0
  while IFS= read -r host; do
    [[ -n "${host}" ]] || continue
    if in_list "${host}" "${CP_SERVER_IDS}" || in_list "${host}" "${WORKER_SERVER_IDS}"; then
      FOUND=$((FOUND + 1))
    fi
  done < <($KUBECTL get edgehosts \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)

  [[ $FOUND -ge $EXPECTED ]] && break

  [[ $ELAPSED -ge $TIMEOUT ]] && {
    >&2 echo "ERROR: Only ${FOUND}/${EXPECTED} EdgeHosts registered after ${TIMEOUT}s"
    $KUBECTL get edgehosts \
      -o jsonpath='{range .items[*]}{.metadata.name}{"\t stage="}{.status.conditions[0].message}{"\n"}{end}' >&2 || true
    exit 1
  }

  >&2 echo "    ${FOUND}/${EXPECTED} EdgeHosts found, waiting ${INTERVAL}s..."
  $KUBECTL get edgehosts \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\t stage="}{.status.conditions[0].message}{"\n"}{end}' >&2 || true
  sleep $INTERVAL
  ELAPSED=$((ELAPSED + INTERVAL))
done

>&2 echo "==> All ${EXPECTED} EdgeHosts registered."

# --- Create EdgeCluster ------------------------------------------------------

MANIFEST=$(mktemp)
cat > "${MANIFEST}" <<YAML
apiVersion: edge.stackit.cloud/v1alpha1
kind: EdgeCluster
metadata:
  name: ${CLUSTER_NAME}
  namespace: default
spec:
  proxy:
    kubernetes: true
    talos: true
  talos:
    version: "${TALOS_VERSION}"
    kubernetes:
      version: "${KUBERNETES_VERSION}"
  maintenance: {}
  nodes:
YAML

for id in "${CP_IDS[@]}"; do
  cat >> "${MANIFEST}" <<YAML
    - edgeHost: ${id}
      role: controlplane
      installDisk: /dev/vda
      maintenance: {}
      proxyConfig: {}
      talos: {}
YAML
done

for id in "${WORKER_IDS[@]}"; do
  cat >> "${MANIFEST}" <<YAML
    - edgeHost: ${id}
      role: worker
      installDisk: /dev/vda
      maintenance: {}
      proxyConfig: {}
      talos: {}
YAML
done

$KUBECTL apply -f "${MANIFEST}"
rm "${MANIFEST}"

# --- Wait for Ready ----------------------------------------------------------

>&2 echo "==> Waiting for EdgeCluster to become ready (timeout: 30 min)..."
TIMEOUT=1800 INTERVAL=20 ELAPSED=0

while true; do
  READY=$($KUBECTL get edgecluster "${CLUSTER_NAME}" \
    -o jsonpath='{range .status.conditions[?(@.type=="ClusterReady")]}{.status}{end}' \
    2>/dev/null || true)

  [[ "${READY}" == "True" ]] && { >&2 echo "==> ClusterReady."; break; }

  [[ $ELAPSED -ge $TIMEOUT ]] && {
    >&2 echo "ERROR: EdgeCluster not ready after ${TIMEOUT}s"
    $KUBECTL get edgecluster "${CLUSTER_NAME}" -o yaml >&2 || true
    exit 1
  }

  >&2 echo "    ClusterReady=${READY:-unknown}, waiting ${INTERVAL}s..."
  sleep $INTERVAL
  ELAPSED=$((ELAPSED + INTERVAL))
done

# --- Extract credentials -----------------------------------------------------

$KUBECTL get secret "${CLUSTER_NAME}-kubeconfig" -n default \
  -o jsonpath='{.data.value}' | base64 --decode \
  > "${OUTPUT_DIR}/${CLUSTER_NAME}.kubeconfig.yaml"
chmod 600 "${OUTPUT_DIR}/${CLUSTER_NAME}.kubeconfig.yaml"

$KUBECTL get secret "${CLUSTER_NAME}-talosconfig" -n default \
  -o jsonpath='{.data.talosconfig}' | base64 --decode \
  > "${OUTPUT_DIR}/${CLUSTER_NAME}.talosconfig.yaml"
chmod 600 "${OUTPUT_DIR}/${CLUSTER_NAME}.talosconfig.yaml"

>&2 echo "==> Cluster ready."
>&2 echo "    kubeconfig:  ${OUTPUT_DIR}/${CLUSTER_NAME}.kubeconfig.yaml"
>&2 echo "    talosconfig: ${OUTPUT_DIR}/${CLUSTER_NAME}.talosconfig.yaml"
