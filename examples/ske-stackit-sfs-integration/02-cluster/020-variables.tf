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

#
# Custom User Settings
#
# Project and mount path are read from the state of 01-storage, see
# 025-storage-state.tf. Set them here only to point somewhere else.
#

variable "storage_state_path" {
  type        = string
  description = "Path to the state file of 01-storage, relative to this directory"
  default     = "../01-storage/terraform.tfstate"
}

variable "stackit_project_id" {
  type        = string
  description = "Project the cluster is created in. Must be the project that holds the SFS share."
  default     = null
}

# Form: "10.2.1.1:/rp_VKL20Ub/nfs-share". Checked in 050-csi.tf.
variable "sfs_mount_path" {
  type        = string
  description = "Mount path of the SFS share, as the NFS server, a colon and the export path"
  default     = null
}

variable "stackit_region" {
  type        = string
  description = "STACKIT region"
  default     = "eu01"
}

variable "stackit_service_account_key_path" {
  type        = string
  description = "Path to the service account key file. Unset falls back to the STACKIT_SERVICE_ACCOUNT_KEY_PATH environment variable, then to that entry in the credentials file ($HOME/.stackit/credentials.json, or STACKIT_CREDENTIALS_PATH)."
  default     = null
}

#
# Cluster network
#

variable "network_name" {
  type        = string
  description = "Name of the network the SKE cluster runs in"
  default     = "ske-example"
}

variable "network_ipv4_prefix_length" {
  type        = number
  description = "Prefix length the SKE network takes out of the network area range"
  default     = 24
}

variable "network_ipv4_nameservers" {
  type        = list(string)
  description = "Nameservers of the network the SKE cluster runs in"
  default     = ["9.9.9.9"]
}

#
# SKE cluster
#

variable "ske_cluster_name" {
  type        = string
  description = "Name of the SKE cluster"
  default     = "sfs"
}

variable "kubernetes_version_min" {
  type        = string
  description = "Minimum Kubernetes version of the SKE cluster. Unset creates the cluster on the latest version SKE supports. Cluster creation fails once SKE no longer offers the pinned version."
  default     = null
}

variable "ske_machine_type" {
  type        = string
  description = "Machine type of the SKE node pool"
  default     = "c2i.2"
}

variable "ske_availability_zones" {
  type        = list(string)
  description = "Availability zones of the SKE node pool"
  default     = ["eu01-3"]
}

# Two nodes, so the demo mounts the share from more than one node at once.
variable "ske_node_pool_minimum" {
  type        = number
  description = "Minimum number of nodes in the SKE node pool"
  default     = 2

  validation {
    condition     = var.ske_node_pool_minimum >= 2
    error_message = "The demo needs at least two nodes to mount the share from more than one node."
  }
}

variable "ske_node_pool_maximum" {
  type        = number
  description = "Maximum number of nodes in the SKE node pool"
  default     = 3
}

#
# NFS CSI driver
#

variable "csi_driver_nfs_version" {
  type        = string
  description = "Chart version of csi-driver-nfs"
  default     = "4.13.4"
}

# The bundled manifests reference this name literally. Change both
# PersistentVolumeClaim.yaml and example-rwx-deployment.yaml if you change it.
variable "storage_class_name" {
  type        = string
  description = "Name of the StorageClass backed by the SFS share"
  default     = "nfs-client"
}

variable "nfs_mount_options" {
  type        = list(string)
  description = "Mount options of the StorageClass"
  default     = ["nfsvers=4.1"]
}
