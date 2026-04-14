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

resource "helm_release" "ingress_nginx" {
  name       = "ingress-nginx"
  repository = "https://kubernetes.github.io/ingress-nginx"
  chart      = "ingress-nginx"
  version    = "4.12.2"
  namespace  = kubernetes_namespace.nginx.metadata[0].name

  values = [
    <<EOF
controller:
  config:
    use-proxy-protocol: "true"
    allow-snippet-annotations: "true"
    compute-full-forwarded-for: "true"
    use-proxy-protocol: "true"
    use-forwarded-headers: "true"
    enable-real-ip: "true"
    forwarded-for-header: "proxy_protocol"
    proxy-connect-timeout: "10"
    proxy-next-upstream: "error timeout http_502 http_503 http_504"
    proxy-next-upstream-timeout: "10"
    proxy-next-upstream-tries: "5"
    retry-non-idempotent: "true"
    proxy-body-size: "5M"
    client-body-buffer-size: "128K"
  replicaCount: 1
  service:
    type: LoadBalancer
    externalTrafficPolicy: Local
    ipFamilyPolicy: SingleStack
    ipFamilies:
      - IPv4
    annotations:
      lb.stackit.cloud/external-address: ${stackit_public_ip.public_ip.ip}
      lb.stackit.cloud/tcp-proxy-protocol: "true"
EOF
  ]

  timeout = 600
}
