# Self-Managed k3s on STACKIT IaaS

> **This example exists solely to validate STACKIT's open-source Kubernetes controllers on a self-managed cluster. It is not a production blueprint.**
>
> Running your own Kubernetes means you own etcd backups, control-plane upgrades, OS patching, certificate rotation, and monitoring. For any real workload, use **[STACKIT Kubernetes Engine (SKE)](https://docs.stackit.cloud/stackit/en/getting-started-kubernetes-75137142.html)** — a fully managed service covered by STACKIT SLAs. The same controllers validated here install identically on SKE: see [`ske-alb-controller-waf-self-installed`](../ske-alb-controller-waf-self-installed).

Deploys a 6-node k3s + Cilium cluster on STACKIT VMs and validates: L4 NLB, L7 ALB, CSI block storage, Cilium NetworkPolicy, and egress connectivity.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
# Set stackit_project_id and stackit_service_account_key_path
terraform init && terraform apply
```

Apply completes in ~10-15 minutes. Verification commands are printed at the end of the Terraform output.

## Accessing the cluster

```bash
ssh -i ~/.ssh/id_rsa ubuntu@<CP1_PUBLIC_IP> 'cat ~/.kube/config' > kubeconfig.yaml
export KUBECONFIG=$PWD/kubeconfig.yaml
sudo kubectl get nodes -o wide
```

## Verifying integrations

**L4 Load Balancer (NLB)**

```bash
sudo kubectl get svc -n test-l4-lb test-nlb-svc -o wide   # wait for EXTERNAL-IP
curl http://$(sudo kubectl get svc -n test-l4-lb test-nlb-svc -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

**L7 Application Load Balancer (ALB)**

```bash
sudo kubectl get ingress -n test-alb hello -o wide         # wait for ADDRESS
ALB_IP=$(sudo kubectl get ingress -n test-alb hello -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl -H "Host: test-alb.example.com" http://$ALB_IP/
```

**CSI Block Storage**

```bash
sudo kubectl get pvc -n test-csi                           # expected: Bound
sudo kubectl exec -n test-csi test-csi-pod -- cat /data/test.txt
```

**Cilium NetworkPolicy**

Two Jobs run automatically. Check after ~90 s:

```bash
sudo kubectl get jobs -n test-cilium
sudo kubectl logs -n test-cilium job/allowed-client        # expect: PASS: reached web-server
sudo kubectl logs -n test-cilium job/blocked-client        # expect: PASS: blocked by Cilium
```

**Egress / debug (netshoot DaemonSet)**

One `nicolaka/netshoot` pod runs on every node for live egress testing:

```bash
sudo kubectl get pods -n test-egress -o wide               # one pod per node
sudo kubectl exec -n test-egress -it <pod> -- bash

# Inside the shell:
curl -s https://ifconfig.me                           # external egress + TLS
dig google.com                                        # DNS
curl -s http://web-server.test-cilium/                # intra-cluster
```

## Version compatibility

| k3s version    | `ccm_branch`    |
| -------------- | --------------- |
| `v1.34.x+k3s1` | `release-v1.34` |
| `v1.35.x+k3s1` | `release-v1.35` |
| `v1.36.x+k3s1` | `release-v1.36` |

## Customisation

| Variable               | Default                                                            |
| ---------------------- | ------------------------------------------------------------------ |
| `k3s_version`          | `v1.36.3+k3s1`                                                     |
| `ccm_branch`           | `release-v1.36`                                                    |
| `cp_machine_type`      | `c2i.4`                                                            |
| `worker_machine_type`  | `c2i.4`                                                            |
| `alb_controller_image` | `ghcr.io/stackitcloud/application-load-balancer-controller:v0.2.0` |

## Cleanup

```bash
terraform destroy
```

> **Note:** STACKIT NLBs and ALBs provisioned by the controllers are not tracked by Terraform state. Delete them via the STACKIT Portal or CLI before `terraform destroy`, or they will block network deletion.
