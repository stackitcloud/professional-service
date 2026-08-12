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

locals {
  bootstrap_script = templatefile("${path.module}/scripts/bootstrap.sh.tftpl", {
    project_id           = var.stackit_project_id
    network_id           = stackit_network.cluster.network_id
    region               = var.stackit_region
    cp1_public_ip        = stackit_public_ip.cp1.ip
    sa_key_b64           = base64encode(stackit_service_account_key.cluster.json)
    ccm_repo_url         = var.ccm_repo_url
    ccm_branch           = var.ccm_branch
    alb_controller_image = var.alb_controller_image
  })
}

# Bootstraps the Kubernetes layer after all VMs and cloud-init are done.
# Connects to CP1, waits for the full cluster, then installs:
#   - cloud-provider-stackit (CCM + CSI driver)
#   - CSI snapshot controller
#   - STACKIT ALB ingress controller
#   - test workloads (L4 LB, CSI PVC, Cilium NetworkPolicy, L7 ALB Ingress)
resource "null_resource" "k8s_bootstrap" {
  depends_on = [
    stackit_server.cp,
    stackit_server.worker,
    stackit_authorization_project_role_assignment.nlb_admin,
    stackit_authorization_project_role_assignment.blockstorage_admin,
    stackit_authorization_project_role_assignment.alb_admin,
    stackit_authorization_project_role_assignment.dns_admin,
  ]

  # Re-run bootstrap if the SA key or k3s token changes.
  triggers = {
    sa_key_id  = stackit_service_account_key.cluster.id
    k3s_token  = random_password.k3s_token.result
    ccm_branch = var.ccm_branch
  }

  connection {
    type        = "ssh"
    host        = stackit_public_ip.cp1.ip
    user        = "ubuntu"
    private_key = file(var.ssh_private_key_path)
    timeout     = "10m"
  }

  # Wait for cloud-init on CP1 to finish (installs k3s + Cilium — takes ~5 min).
  # || true: cloud-init exits non-zero if any runcmd step fails (e.g. Helm timeout);
  # we still want to proceed — the bootstrap script handles its own readiness checks.
  provisioner "remote-exec" {
    inline = ["cloud-init status --wait || true"]
  }

  # Transfer test manifests to CP1.
  # The file provisioner requires the destination directory to already exist.
  provisioner "remote-exec" {
    inline = ["mkdir -p /tmp/manifests"]
  }

  provisioner "file" {
    source      = "${path.module}/manifests/"
    destination = "/tmp/manifests"
  }

  # Transfer and run the bootstrap script.
  provisioner "file" {
    content     = local.bootstrap_script
    destination = "/tmp/bootstrap.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/bootstrap.sh",
      "sudo /tmp/bootstrap.sh",
    ]
  }
}
