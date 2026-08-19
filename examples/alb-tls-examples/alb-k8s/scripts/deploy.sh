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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

KUBECONFIG="$(realpath "${KUBECONFIG:-${REPO_ROOT}/.kubeconfig}")"
export KUBECONFIG

if [[ ! -f "${KUBECONFIG}" ]]; then
  echo "Error: kubeconfig not found at ${KUBECONFIG}"
  exit 1
fi

echo "Using kubeconfig: ${KUBECONFIG}"

HELM="helm --kubeconfig ${KUBECONFIG}"
KUBECTL="kubectl --kubeconfig ${KUBECONFIG}"

TERRAFORM_DIR="${REPO_ROOT}/terraform"

echo "==> [1/6] Traefik Ingress Controller"
helm repo add traefik https://traefik.github.io/charts
helm repo update traefik
${HELM} upgrade --install traefik traefik/traefik \
  --namespace traefik \
  --create-namespace \
  --set "ports.web.redirectTo.port=websecure" \
  --set service.type=LoadBalancer \
  --wait --timeout 5m

echo "    Waiting for LoadBalancer IP..."
for i in $(seq 1 24); do
  LB_IP=$(${KUBECTL} get svc -n traefik traefik \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  [[ -n "${LB_IP}" ]] && break
  sleep 5
done

if [[ -z "${LB_IP:-}" ]]; then
  echo "Error: LoadBalancer IP not assigned after 2 minutes."
  exit 1
fi
echo "    LoadBalancer IP: ${LB_IP}"

echo "==> [2/6] DNS A record"
PROJECT_ID=$(cd "${TERRAFORM_DIR}" && terraform output -raw project_id)
ZONE_ID=$(cd "${TERRAFORM_DIR}" && terraform output -raw dns_zone_id)
APP_FQDN=$(cd "${TERRAFORM_DIR}" && terraform output -raw app_fqdn)
APP_HOSTNAME="${APP_FQDN%%.*}"

RS_ID=$(stackit dns record-set list \
  --project-id "${PROJECT_ID}" \
  --zone-id "${ZONE_ID}" \
  -o json 2>/dev/null | \
  jq -r --arg name "${APP_FQDN}." '.[] | select(.name==$name and .type=="A") | .id' 2>/dev/null || true)

if [[ -n "${RS_ID}" ]]; then
  echo "    DNS A record already exists, skipping."
else
  stackit dns record-set create \
    --project-id "${PROJECT_ID}" \
    --zone-id "${ZONE_ID}" \
    --name "${APP_HOSTNAME}" \
    --type A \
    --record "${LB_IP}" \
    --ttl 300 \
    --async \
    -y
  echo "    DNS A record created: ${APP_FQDN} → ${LB_IP}"
fi

echo "==> [3/6] cert-manager"
helm repo add jetstack https://charts.jetstack.io
helm repo update jetstack
if ${HELM} status cert-manager --namespace cert-manager &>/dev/null; then
  echo "    cert-manager already installed, skipping."
else
  ${HELM} install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --set crds.enabled=true \
    --wait --timeout 5m
fi

echo "==> [4/6] STACKIT SA secret for cert-manager webhook"
SA_KEY="${REPO_ROOT}/terraform/keys/sa-key.json"
if [[ ! -f "${SA_KEY}" ]]; then
  echo "Error: SA key not found at ${SA_KEY}"
  exit 1
fi
${KUBECTL} create secret generic stackit-sa-authentication \
  --namespace cert-manager \
  --from-file=sa.json="${SA_KEY}" \
  --dry-run=client -o yaml | ${KUBECTL} apply -f -

echo "==> [5/6] STACKIT cert-manager webhook"
${KUBECTL} wait deployment/cert-manager-webhook \
  --namespace cert-manager \
  --for=condition=Available \
  --timeout=120s
helm repo add stackit-cert-manager-webhook \
  https://stackitcloud.github.io/stackit-cert-manager-webhook
helm repo update stackit-cert-manager-webhook
if ${HELM} status stackit-cert-manager-webhook --namespace cert-manager &>/dev/null; then
  echo "    stackit-cert-manager-webhook already installed, skipping."
else
  ${HELM} install stackit-cert-manager-webhook \
    stackit-cert-manager-webhook/stackit-cert-manager-webhook \
    --namespace cert-manager \
    --set stackitSaAuthentication.enabled=true \
    --wait --timeout 5m
fi

echo "==> [6/6] Kubernetes manifests"
${KUBECTL} apply -f "${REPO_ROOT}/kubernetes/cert-manager/01-cluster-issuer.yaml"
${KUBECTL} apply -f "${REPO_ROOT}/kubernetes/nginx/00-namespace.yaml"
${KUBECTL} apply -f "${REPO_ROOT}/kubernetes/nginx/01-deployment.yaml"
${KUBECTL} apply -f "${REPO_ROOT}/kubernetes/nginx/02-service.yaml"
${KUBECTL} apply -f "${REPO_ROOT}/kubernetes/cert-manager/02-certificate.yaml"
${KUBECTL} apply -f "${REPO_ROOT}/kubernetes/nginx/03-ingress.yaml"

echo ""
echo "==> Done. App: https://${APP_FQDN}"
echo "    ${KUBECTL} describe certificate nginx-tls -n nginx-showcase"
