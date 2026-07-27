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

resource "kubernetes_namespace_v1" "this" {
  metadata {
    name = var.namespace
  }
}

# The annotation tells the stackit-pod-identity-webhook which STACKIT service
# account to impersonate. The webhook injects the projected token volume and
# the STACKIT_SERVICE_ACCOUNT_EMAIL environment variable automatically.
resource "kubernetes_service_account_v1" "workload" {
  metadata {
    name      = "workload-identity-demo"
    namespace = kubernetes_namespace_v1.this.metadata[0].name
    annotations = {
      "workload-identity.stackit.cloud/service-account-email" = stackit_service_account.workload.email
    }
  }
}

# Block access to the cloud metadata service. Without this a compromised pod
# could reach 169.254.169.254 to steal node-level credentials.
resource "kubernetes_network_policy_v1" "deny_metadata_access" {
  metadata {
    name      = "deny-metadata-access"
    namespace = kubernetes_namespace_v1.this.metadata[0].name
  }

  spec {
    pod_selector {}
    policy_types = ["Egress"]

    egress {
      to {
        ip_block {
          cidr   = "0.0.0.0/0"
          except = ["169.254.169.254/32"]
        }
      }
    }
  }
}

# https://iaas.api.stackit.cloud/v2/networks/public-ip-ranges
# https://github.com/stackitcloud/stackit-pod-identity-webhook/blob/36260244f5983b397748e29354a672b2710bc87c/cmd/stackit-workload-identity-example-app/main.go#L44
# Demo job: lists available public ip ranges via the STACKIT API using
# only the injected workload identity token — no static credentials required.
resource "kubernetes_job_v1" "demo" {
  metadata {
    name      = "workload-identity-demo"
    namespace = kubernetes_namespace_v1.this.metadata[0].name
  }

  spec {
    template {
      metadata {}
      spec {
        service_account_name = kubernetes_service_account_v1.workload.metadata[0].name
        # Disable the default SA token mount. The webhook injects its own
        # short-lived projected token, making the default token redundant.
        automount_service_account_token = false
        restart_policy                  = "OnFailure"

        container {
          name  = "demo"
          image = "ghcr.io/stackitcloud/stackit-workload-identity-example-app:latest"
        }
      }
    }

    backoff_limit = 4
  }
}
