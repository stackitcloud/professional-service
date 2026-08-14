# STACKIT Edge Cloud Cluster on IaaS

Deploys a STACKIT Edge Cloud (STEC) management plane and a Talos Linux Kubernetes cluster on STACKIT Compute Engine VMs: three control plane nodes and three worker nodes.

## Architecture

```
STACKIT Cloud
├── STEC Management Plane
│   └── EdgeImage: talos-<version>  (plain Talos)
├── IaaS (eu01-1)
│   ├── Network (routed, 10.0.10.0/24)
│   ├── Server: edge-cluster-cp-0    -> EdgeHost -> controlplane
│   ├── Server: edge-cluster-cp-1    -> EdgeHost -> controlplane
│   ├── Server: edge-cluster-cp-2    -> EdgeHost -> controlplane
│   ├── Server: edge-cluster-worker-0 -> EdgeHost -> worker
│   ├── Server: edge-cluster-worker-1 -> EdgeHost -> worker
│   └── Server: edge-cluster-worker-2 -> EdgeHost -> worker
└── EdgeCluster (managed via STEC Kubernetes API)
    └── cloud proxy enabled (kubectl/talosctl over STEC tunnel)
```

EdgeImage and EdgeCluster resources are CRDs on the STEC Kubernetes API, not managed by the STACKIT Terraform provider. Two `local-exec` scripts bridge this gap. EdgeHost names are STACKIT IaaS server UUIDs, so role assignment (controlplane vs. worker) is deterministic and independent of registration order.

## Prerequisites

- `terraform`, `kubectl`, `stackit` CLI, `curl`, `unxz`, `python3`
- Service account with editor access on the project
- `stackit` CLI authenticated (`stackit auth login`)

## Configure

```bash
cp terraform.tfvars.example terraform.tfvars
```

| Variable                           | How to get it                            |
| ---------------------------------- | ---------------------------------------- |
| `stackit_project_id`               | STACKIT Portal or `stackit project list` |
| `stackit_service_account_key_path` | `stackit service-account key create`     |
| `stec_plan_id`                     | `stackit beta edge-cloud plans list`     |

## Deploy

```bash
terraform init
terraform apply
```

Total time: ~25-35 min. After apply:

| File                                    | Contents                         |
| --------------------------------------- | -------------------------------- |
| `.stec.kubeconfig.json`                 | STEC management plane kubeconfig |
| `.generated/<cluster>.kubeconfig.yaml`  | Edge cluster kubeconfig          |
| `.generated/<cluster>.talosconfig.yaml` | Edge cluster talosconfig         |

## Verify

```bash
export KUBECONFIG=.generated/edge-test-cluster.kubeconfig.yaml

kubectl get nodes -o wide
```

## Manage the EdgeCluster

```bash
export KUBECONFIG=.stec.kubeconfig.json

kubectl get edgehosts
kubectl get edgeclusters

# Upgrade Talos or Kubernetes (one at a time, triggers rolling reboot)
kubectl patch edgecluster edge-test-cluster --type=merge \
  -p '{"spec": {"talos": {"version": "v1.14.0-stackit.v1.8.0"}}}'
```

## Cleanup

```bash
# Delete the EdgeCluster before destroying VMs to avoid orphaned EdgeHosts
export KUBECONFIG=.stec.kubeconfig.json
kubectl delete edgecluster edge-test-cluster

terraform destroy
```

## Notes

- **No SSH / no cloud-init**: Talos is configured entirely via the STEC management plane over mTLS.
- **Do not clone VMs**: Talos uses the disk UUID as the EdgeHost identity, cloning breaks cluster membership.
- **STEC kubeconfig expiry**: Valid for 24 h. Re-run `terraform apply` to rotate.
- **etcd HA**: Three control plane nodes provide a quorum-tolerant etcd cluster.
