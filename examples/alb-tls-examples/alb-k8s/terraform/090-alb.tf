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

# A native STACKIT ALB Ingress Controller for Kubernetes does not yet exist (mid-2026).
#
# Current traffic path:
#   Internet → STACKIT NLB (yawol, auto-provisioned) → Traefik IC → Pod
#
# Target path when available:
#   Internet → STACKIT ALB → Kubernetes Service / Ingress → Pod
#
# Expected changes:
#   - ingressClassName: traefik  →  ingressClassName: <stackit-alb>
#   - Traefik IC replaced by ALB as L7 termination point
#   - TLS terminates at the ALB; cert-manager integration TBD
#   - nginx.ingress.kubernetes.io/* annotations replaced by ALB equivalents
#
# For an external ALB in front of the cluster (available today), see ../alb/.

# resource "stackit_application_load_balancer" "this" {
#   project_id       = stackit_resourcemanager_project.this.project_id
#   region           = var.region
#   name             = "${var.cluster_name}-alb"
#   plan_id          = "p10"
#   external_address = "<STACKIT_PUBLIC_IP>"
#
#   listeners = [
#     {
#       name     = "https"
#       port     = 443
#       protocol = "PROTOCOL_HTTPS"
#       https = {
#         certificate_config = {
#           certificate_ids = ["<STACKIT_CERT_ID>"]
#         }
#       }
#       http = {
#         hosts = [{
#           host  = var.app_hostname
#           rules = [{ target_pool = "ske-nodeport" }]
#         }]
#       }
#     }
#   ]
#
#   networks = [
#     {
#       network_id = "<SKE_NETWORK_ID>"
#       role       = "ROLE_LISTENERS_AND_TARGETS"
#     }
#   ]
#
#   target_pools = [
#     {
#       name        = "ske-nodeport"
#       target_port = 30080
#       targets     = []
#     }
#   ]
# }
