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

# Installs cert-manager and trust-manager with Helm.
#
# trust-manager needs cert-manager for its webhook certificate by default, so the
# installation is deliberately staged:
#   1. install or upgrade cert-manager
#   2. wait until cert-manager controllers and Pods are ready
#   3. install or upgrade trust-manager with kube-system as its trust namespace
#   4. wait until trust-manager Pods are ready
#
# The trust namespace is where trust-manager reads source ConfigMaps and
# Secrets. This PoC stores custom-ca-store in kube-system, so the Helm value
# app.trust.namespace must be kube-system.

CERT_MANAGER_NAMESPACE="${CERT_MANAGER_NAMESPACE:-cert-manager}"
TRUST_NAMESPACE="${TRUST_NAMESPACE:-kube-system}"
CERT_MANAGER_RELEASE="${CERT_MANAGER_RELEASE:-cert-manager}"
TRUST_MANAGER_RELEASE="${TRUST_MANAGER_RELEASE:-trust-manager}"
CERT_MANAGER_CHART="${CERT_MANAGER_CHART:-oci://quay.io/jetstack/charts/cert-manager}"
TRUST_MANAGER_CHART="${TRUST_MANAGER_CHART:-oci://quay.io/jetstack/charts/trust-manager}"
TIMEOUT="${TIMEOUT:-300s}"

command -v helm >/dev/null 2>&1 || {
  echo "ERROR: helm is required to install cert-manager and trust-manager." >&2
  exit 1
}

command -v kubectl >/dev/null 2>&1 || {
  echo "ERROR: kubectl is required to verify Kubernetes readiness." >&2
  exit 1
}

echo "Installing or upgrading cert-manager in namespace ${CERT_MANAGER_NAMESPACE}..."
helm upgrade "${CERT_MANAGER_RELEASE}" "${CERT_MANAGER_CHART}" \
  --install \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --create-namespace \
  --set crds.enabled=true \
  --wait \
  --timeout "${TIMEOUT}"

echo "Waiting for cert-manager controller Deployments to finish rolling out..."
kubectl rollout status deployment/cert-manager \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --timeout="${TIMEOUT}"
kubectl rollout status deployment/cert-manager-cainjector \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --timeout="${TIMEOUT}"
kubectl rollout status deployment/cert-manager-webhook \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --timeout="${TIMEOUT}"

echo "Waiting for all cert-manager Pods to report Ready..."
kubectl wait \
  --for=condition=Ready \
  pods \
  --all \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --timeout="${TIMEOUT}"

echo "Installing or upgrading trust-manager with trust namespace ${TRUST_NAMESPACE}..."
helm upgrade "${TRUST_MANAGER_RELEASE}" "${TRUST_MANAGER_CHART}" \
  --install \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --set "app.trust.namespace=${TRUST_NAMESPACE}" \
  --wait \
  --timeout "${TIMEOUT}"

echo "Waiting for trust-manager controller Deployment to finish rolling out..."
kubectl rollout status deployment/trust-manager \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --timeout="${TIMEOUT}"

echo "Waiting for all Pods in ${CERT_MANAGER_NAMESPACE} to report Ready after trust-manager install..."
kubectl wait \
  --for=condition=Ready \
  pods \
  --all \
  --namespace "${CERT_MANAGER_NAMESPACE}" \
  --timeout="${TIMEOUT}"

echo "cert-manager and trust-manager installation complete."
