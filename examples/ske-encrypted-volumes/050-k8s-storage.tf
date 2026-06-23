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

resource "kubernetes_storage_class_v1" "encrypted_premium" {
  metadata {
    name = "stackit-encrypted-premium"
  }

  storage_provisioner    = "block-storage.csi.stackit.cloud"
  reclaim_policy         = "Delete"
  allow_volume_expansion = true
  volume_binding_mode    = "WaitForFirstConsumer"

  parameters = {
    type              = "storage_premium_perf6"
    encrypted         = "true"
    kmsKeyID          = stackit_kms_key.volume_key.key_id
    kmsKeyringID      = stackit_kms_keyring.encryption.keyring_id
    kmsProjectID      = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    kmsKeyVersion     = "1"
    kmsServiceAccount = stackit_service_account.kms_manager.email
  }

  depends_on = [
    stackit_authorization_service_account_role_assignment.ske_impersonation,
    stackit_authorization_project_role_assignment.kms_user
  ]
}

resource "kubernetes_persistent_volume_claim_v1" "test_pvc" {
  metadata {
    name = "test-encryption-pvc"
  }

  spec {
    access_modes = ["ReadWriteOnce"]

    resources {
      requests = {
        storage = "10Gi"
      }
    }

    storage_class_name = kubernetes_storage_class_v1.encrypted_premium.metadata[0].name
  }
}

resource "kubernetes_pod_v1" "test_app" {
  metadata {
    name = "encrypted-volume-test"
  }

  spec {
    container {
      image = "nginx:latest"
      name  = "web-server"

      volume_mount {
        mount_path = "/usr/share/nginx/html"
        name       = "data-volume"
      }
    }

    volume {
      name = "data-volume"
      persistent_volume_claim {
        claim_name = "test-encryption-pvc"
      }
    }
  }
}
