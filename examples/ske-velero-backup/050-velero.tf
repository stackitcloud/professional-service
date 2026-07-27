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

resource "kubernetes_namespace_v1" "velero" {
  metadata {
    name = "velero"
  }
}

# STACKIT Object Storage uses the AWS S3 API. Velero's AWS plugin expects the
# credentials in a file with the standard AWS credentials format, stored under
# the "cloud" key of a Kubernetes secret.
resource "kubernetes_secret_v1" "velero_s3_credentials" {
  metadata {
    name      = "velero-s3-credentials"
    namespace = kubernetes_namespace_v1.velero.metadata[0].name
  }

  data = {
    cloud = <<-EOT
      [default]
      aws_access_key_id=${stackit_objectstorage_credential.velero.access_key}
      aws_secret_access_key=${stackit_objectstorage_credential.velero.secret_access_key}
    EOT
  }
}

resource "helm_release" "velero" {
  name       = "velero"
  namespace  = kubernetes_namespace_v1.velero.metadata[0].name
  repository = "https://vmware-tanzu.github.io/helm-charts"
  chart      = "velero"
  version    = "12.1.0"
  lint       = true

  # Wait for the CRDs and deployment to be fully ready before Terraform
  # considers the release complete, so dependent resources (e.g. Schedule
  # manifests) can be created immediately after.
  wait    = true
  timeout = 300

  depends_on = [helm_release.kube_prometheus_stack]

  values = [
    yamlencode({
      resources = {
        requests = {
          cpu    = "500m"
          memory = "512Mi"
        }
        limits = {
          memory = "2Gi"
        }
      }

      containerSecurityContext = {
        readOnlyRootFilesystem = true
      }

      initContainers = [
        {
          name            = "velero-plugin-for-aws"
          image           = "velero/velero-plugin-for-aws:v1.14.2"
          imagePullPolicy = "IfNotPresent"
          volumeMounts = [
            {
              name      = "plugins"
              mountPath = "/target"
            }
          ]
          securityContext = {
            readOnlyRootFilesystem = true
          }
          resources = {
            requests = {
              cpu    = "500m"
              memory = "512Mi"
            }
            limits = {
              memory = "1Gi"
            }
          }
        }
      ]

      # STACKIT Object Storage does not expose the CSI snapshot API.
      snapshotsEnabled = false
      backupsEnabled   = true
      deployNodeAgent  = false

      credentials = {
        useSecret      = true
        existingSecret = kubernetes_secret_v1.velero_s3_credentials.metadata[0].name
      }

      configuration = {
        backupStorageLocation = [
          {
            name     = "default"
            provider = "aws"
            bucket   = stackit_objectstorage_bucket.velero.name
            default  = true
            config = {
              region           = var.stackit_region
              s3ForcePathStyle = "true"
              s3Url            = "https://object.storage.${var.stackit_region}.onstackit.cloud"
              # STACKIT Object Storage does not support checksum trailers;
              # setting this to an empty string disables the header entirely.
              checksumAlgorithm = ""
            }
          }
        ]
      }

      upgradeCRDs = true

      kubectl = {
        image = {
          tag = "1.34"
        }
        resources = {
          requests = {
            cpu    = "500m"
            memory = "512Mi"
          }
          limits = {
            memory = "1Gi"
          }
        }
        containerSecurityContext = {
          readOnlyRootFilesystem = true
        }
      }

      upgradeJobResources = {
        requests = {
          cpu    = "500m"
          memory = "512Mi"
        }
        limits = {
          memory = "1Gi"
        }
      }

      metrics = {
        enabled       = true
        scrapeTimeout = "10s"
        serviceMonitor = {
          autodetect       = false
          enabled          = true
          namespace        = kubernetes_namespace_v1.velero.metadata[0].name
          annotations      = {}
          additionalLabels = {}
        }
        nodeAgentPodMonitor = {
          autodetect       = true
          enabled          = true
          annotations      = {}
          additionalLabels = {}
        }
      }

      schedules = {
        "full-cluster-backup" = {
          disabled                   = false
          schedule                   = var.velero_backup_schedule
          useOwnerReferencesInBackup = false
          paused                     = false
          template = {
            ttl                              = var.velero_backup_ttl
            storageLocation                  = "default"
            includedNamespaces               = ["*"]
            excludedNamespaceScopedResources = ["persistentVolumeClaims"]
            excludedClusterScopedResources   = ["persistentVolumes"]
          }
        }
      }
    })
  ]
}
