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

# This file defines the required Terraform providers and their configurations.
# It sets up the STACKIT, Kubernetes, and Helm providers to manage resources in the project and the SKE cluster.
terraform {
  required_version = ">= 0.14.0"
  required_providers {
    stackit = {
      source  = "stackitcloud/stackit"
      version = ">= 0.87.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.6.3"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">=2.14.0"
    }
  }
}

provider "stackit" {
  default_region           = var.stackit_region
  service_account_key_path = var.stackit_service_account_key_path
  experiments              = ["iam"]
}

provider "kubernetes" {
  host                   = yamldecode(stackit_ske_kubeconfig.this.kube_config).clusters.0.cluster.server
  client_certificate     = base64decode(yamldecode(stackit_ske_kubeconfig.this.kube_config).users.0.user.client-certificate-data)
  client_key             = base64decode(yamldecode(stackit_ske_kubeconfig.this.kube_config).users.0.user.client-key-data)
  cluster_ca_certificate = base64decode(yamldecode(stackit_ske_kubeconfig.this.kube_config).clusters.0.cluster.certificate-authority-data)
}

provider "helm" {
  kubernetes = {
    host                   = yamldecode(stackit_ske_kubeconfig.this.kube_config).clusters.0.cluster.server
    client_certificate     = base64decode(yamldecode(stackit_ske_kubeconfig.this.kube_config).users.0.user.client-certificate-data)
    client_key             = base64decode(yamldecode(stackit_ske_kubeconfig.this.kube_config).users.0.user.client-key-data)
    cluster_ca_certificate = base64decode(yamldecode(stackit_ske_kubeconfig.this.kube_config).clusters.0.cluster.certificate-authority-data)
  }
}
