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

# This script creates a local fake Root CA and a server certificate for the
# dummy HTTPS Service. It then installs the server certificate as a Kubernetes
# TLS Secret and renders the Root CA into the node-level ConfigMap manifest that
# later phases will apply.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BLUEPRINT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
NODE_LEVEL_DIR="${BLUEPRINT_DIR}/01-node-level"

CA_KEY="${SCRIPT_DIR}/fake-ca.key"
CA_CERT="${SCRIPT_DIR}/fake-ca.crt"
SERVER_KEY="${SCRIPT_DIR}/tls.key"
SERVER_CSR="${SCRIPT_DIR}/tls.csr"
SERVER_CERT="${SCRIPT_DIR}/tls.crt"
OPENSSL_EXT="$(mktemp)"

cleanup() {
  rm -f "${OPENSSL_EXT}" "${SERVER_CSR}"
}
trap cleanup EXIT

command -v openssl >/dev/null 2>&1 || {
  echo "ERROR: openssl is required to generate certificates." >&2
  exit 1
}

command -v kubectl >/dev/null 2>&1 || {
  echo "ERROR: kubectl is required to create Kubernetes resources." >&2
  exit 1
}

mkdir -p "${NODE_LEVEL_DIR}"

echo "Generating fake Root CA..."
openssl genrsa -out "${CA_KEY}" 4096
openssl req \
  -x509 \
  -new \
  -nodes \
  -key "${CA_KEY}" \
  -sha256 \
  -days 3650 \
  -out "${CA_CERT}" \
  -subj "/CN=Fake Kubernetes Root CA"

echo "Generating server certificate for dummy-server.default.svc.cluster.local..."
openssl genrsa -out "${SERVER_KEY}" 2048
openssl req \
  -new \
  -key "${SERVER_KEY}" \
  -out "${SERVER_CSR}" \
  -subj "/CN=dummy-server.default.svc.cluster.local"

# The Subject Alternative Name is required by modern TLS clients. The Service
# DNS names below cover the fully qualified name used by the tests and common
# shorter in-cluster aliases.
cat >"${OPENSSL_EXT}" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=@alt_names

[alt_names]
DNS.1=dummy-server
DNS.2=dummy-server.default
DNS.3=dummy-server.default.svc
DNS.4=dummy-server.default.svc.cluster.local
EOF

openssl x509 \
  -req \
  -in "${SERVER_CSR}" \
  -CA "${CA_CERT}" \
  -CAkey "${CA_KEY}" \
  -CAcreateserial \
  -out "${SERVER_CERT}" \
  -days 825 \
  -sha256 \
  -extfile "${OPENSSL_EXT}"

echo "Creating or updating Kubernetes Secret default/dummy-tls..."
kubectl create secret tls dummy-tls \
  --namespace default \
  --cert="${SERVER_CERT}" \
  --key="${SERVER_KEY}" \
  --dry-run=client \
  -o yaml | kubectl apply -f -

echo "Rendering ${NODE_LEVEL_DIR}/configmap-ca.yaml from the generated Root CA..."
kubectl create configmap custom-ca-store \
  --namespace kube-system \
  --from-file=fake-ca.crt="${CA_CERT}" \
  --dry-run=client \
  -o yaml >"${NODE_LEVEL_DIR}/configmap-ca.yaml"

echo "Certificate generation complete."
echo "Generated Root CA: ${CA_CERT}"
echo "Generated server certificate: ${SERVER_CERT}"
echo "Generated node-level ConfigMap manifest: ${NODE_LEVEL_DIR}/configmap-ca.yaml"
