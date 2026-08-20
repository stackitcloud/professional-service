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
STACKIT_PROJECT_ID="${STACKIT_PROJECT_ID:?}"
STACKIT_REGION="${STACKIT_REGION:?}"
TALOS_VERSION="${TALOS_VERSION:?}"
IMAGE_NAME="${IMAGE_NAME:?}"
DOWNLOAD_DIR="${DOWNLOAD_DIR:?}"
OUTPUT_DIR="${OUTPUT_DIR:?}"
DISK_SIZE_GB="${DISK_SIZE_GB:-100}"

KUBECTL="kubectl --kubeconfig=${KUBECONFIG}"
IMAGE_IDS_FILE="${OUTPUT_DIR}/image-ids.json"

mkdir -p "${DOWNLOAD_DIR}" "${OUTPUT_DIR}"

# Skip if image ID is already recorded.
if [[ -f "${IMAGE_IDS_FILE}" ]]; then
  IMAGE_ID=$(python3 -c "import json; d=json.load(open('${IMAGE_IDS_FILE}')); print(d.get('image_id',''))" 2>/dev/null || true)
  if [[ -n "${IMAGE_ID}" ]]; then
    >&2 echo "==> Image ID already recorded (${IMAGE_ID}), skipping."
    cat "${IMAGE_IDS_FILE}"
    exit 0
  fi
fi

# --- EdgeImage CRD ----------------------------------------------------------

>&2 echo "==> Applying EdgeImage '${IMAGE_NAME}'..."
$KUBECTL apply -f - <<EOF
apiVersion: edge.stackit.cloud/v1alpha1
kind: EdgeImage
metadata:
  name: ${IMAGE_NAME}
  namespace: default
spec:
  talosVersion: "${TALOS_VERSION}"
  schematic: |
    customization:
      extraKernelArgs: []
      systemExtensions:
        officialExtensions: []
    overlay: {}
EOF

# --- Wait for ready ---------------------------------------------------------
# EdgeImage has no .status.phase field. Readiness is signalled by
# .status.artifacts being populated with the openstack/amd64 artifact URL.

get_artifact_url() {
  $KUBECTL get edgeimage "${IMAGE_NAME}" -o json 2>/dev/null | python3 -c "
import json, sys
for s in json.load(sys.stdin).get('status', {}).get('artifacts', []):
    if s.get('type') == 'openstack':
        for a in s.get('artifacts', []):
            if a.get('architecture') == 'amd64':
                print(a['url']); sys.exit(0)
sys.exit(1)
" 2>/dev/null || true
}

>&2 echo "==> Waiting for EdgeImage '${IMAGE_NAME}' to be ready (timeout: 10 min)..."
TIMEOUT=600 INTERVAL=15 ELAPSED=0
while true; do
  URL=$(get_artifact_url)
  if [[ -n "${URL}" ]]; then
    >&2 echo "    Ready. URL: ${URL}"
    break
  fi
  [[ $ELAPSED -ge $TIMEOUT ]] && {
    >&2 echo "ERROR: '${IMAGE_NAME}' not ready after ${TIMEOUT}s"
    $KUBECTL get edgeimage "${IMAGE_NAME}" -o yaml >&2 || true
    exit 1
  }
  >&2 echo "    Not ready yet, retrying in ${INTERVAL}s... (${ELAPSED}/${TIMEOUT}s)"
  sleep $INTERVAL
  ELAPSED=$((ELAPSED + INTERVAL))
done

# --- Download ---------------------------------------------------------------

RAW_XZ="${DOWNLOAD_DIR}/talos.raw.xz"
RAW="${DOWNLOAD_DIR}/talos.raw"

if [[ -f "${RAW}" && -s "${RAW}" ]]; then
  >&2 echo "==> talos.raw exists, skipping download."
else
  # Download to a .part file and rename only on success
  if [[ -f "${RAW_XZ}" && -s "${RAW_XZ}" ]]; then
    >&2 echo "==> talos.raw.xz exists, skipping download."
  else
    >&2 echo "==> Downloading ${IMAGE_NAME}..."
    curl --fail --progress-bar -L -o "${RAW_XZ}.part" "${URL}"
    mv "${RAW_XZ}.part" "${RAW_XZ}"
  fi

  # Decompress with --decompress, to prevent in-place operations of unxz, rm afterwards
  >&2 echo "==> Decompressing..."
  xz --decompress --stdout "${RAW_XZ}" >"${RAW}.part"
  mv "${RAW}.part" "${RAW}"
  rm -f "${RAW_XZ}"
fi

# --- Upload to STACKIT IaaS -------------------------------------------------

EXISTING_ID=$(stackit image list \
  --project-id "${STACKIT_PROJECT_ID}" \
  --region "${STACKIT_REGION}" \
  --output-format json 2>/dev/null \
  | python3 -c "
import json, sys
items = json.load(sys.stdin)
items = items if isinstance(items, list) else items.get('items', [])
for img in items:
    if img.get('name') == '${IMAGE_NAME}':
        print(img.get('id', '')); sys.exit(0)
" 2>/dev/null || true)

if [[ -n "${EXISTING_ID}" ]]; then
  >&2 echo "==> Image '${IMAGE_NAME}' already exists (${EXISTING_ID})."
  IMAGE_ID="${EXISTING_ID}"
else
  >&2 echo "==> Uploading '${IMAGE_NAME}'..."
  RESULT=$(stackit image create \
    --project-id "${STACKIT_PROJECT_ID}" \
    --region "${STACKIT_REGION}" \
    --name "${IMAGE_NAME}" \
    --disk-format raw \
    --local-file-path "${RAW}" \
    --min-disk-size "${DISK_SIZE_GB}" \
    --min-ram 4096 \
    --output-format json \
    --assume-yes)
  IMAGE_ID=$(echo "${RESULT}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
  >&2 echo "    Uploaded: ${IMAGE_ID}"
fi

# --- Write image ID ---------------------------------------------------------

python3 -c "
import json
with open('${IMAGE_IDS_FILE}', 'w') as f:
    json.dump({'image_id': '${IMAGE_ID}'}, f)
"
>&2 echo "==> Image ID written to ${IMAGE_IDS_FILE}"
cat "${IMAGE_IDS_FILE}"
