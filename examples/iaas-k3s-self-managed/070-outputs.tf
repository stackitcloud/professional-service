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

output "cp1_public_ip" {
  description = "Public IP of the first control plane node (API server + SSH entry point)."
  value       = stackit_public_ip.cp1.ip
}

output "cp_private_ips" {
  description = "Private IPs of all control plane nodes."
  value       = stackit_network_interface.cp[*].ipv4
}

output "worker_private_ips" {
  description = "Private IPs of all worker nodes."
  value       = stackit_network_interface.worker[*].ipv4
}

output "ssh_command" {
  description = "SSH into the first control plane node."
  value       = "ssh -i ${var.ssh_private_key_path} ubuntu@${stackit_public_ip.cp1.ip}"
  sensitive   = true
}

output "get_kubeconfig_command" {
  description = "Fetch the kubeconfig from CP1 and save it locally."
  value       = "ssh -o StrictHostKeyChecking=no -i ${var.ssh_private_key_path} ubuntu@${stackit_public_ip.cp1.ip} 'cat ~/.kube/config' > kubeconfig.yaml && echo 'export KUBECONFIG=$PWD/kubeconfig.yaml'"
  sensitive   = true
}

output "test_nodes" {
  description = "Verify all 6 nodes are Ready."
  value       = "KUBECONFIG=kubeconfig.yaml kubectl get nodes -o wide"
}

output "test_l4_loadbalancer" {
  description = "Check that the STACKIT NLB was provisioned for the test-l4-lb Service."
  value       = "KUBECONFIG=kubeconfig.yaml kubectl get svc -n test-l4-lb test-nlb-svc -o wide"
}

output "test_csi" {
  description = "Check that the CSI driver provisioned the PersistentVolumeClaim."
  value       = "KUBECONFIG=kubeconfig.yaml kubectl get pvc -n test-csi && kubectl get pv"
}

output "test_cilium" {
  description = "Check Cilium status and NetworkPolicy enforcement."
  value       = "KUBECONFIG=kubeconfig.yaml kubectl get pods -n test-cilium && kubectl describe networkpolicy -n test-cilium"
}

output "test_alb" {
  description = "Check that the ALB controller created an Application Load Balancer for the Ingress."
  value       = "KUBECONFIG=kubeconfig.yaml kubectl get ingress -n test-alb -o wide"
}

output "cluster_network_id" {
  description = "STACKIT network ID — referenced in the cloud provider ConfigMap."
  value       = stackit_network.cluster.network_id
}
