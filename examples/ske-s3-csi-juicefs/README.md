<!-- tags: ske, juicefs, csi-driver, s3, object-storage, rwx, read-write-many, redis, kubernetes -->

# SKE S3 CSI with JuiceFS

Mounts STACKIT Object Storage as a `ReadWriteMany` Kubernetes volume on SKE using the [JuiceFS CSI driver](https://github.com/juicedata/juicefs-csi-driver).

> S3-backed volumes are slower than block storage. Good fits: shared scratch space, ML dataset mounts, large file ingestion. Not recommended for latency-sensitive workloads.

## What gets deployed

| File                    | Resources                                                                           |
| ----------------------- | ----------------------------------------------------------------------------------- |
| `030-ske.tf`            | SKE cluster + kubeconfig                                                            |
| `040-object-storage.tf` | S3 bucket + credentials                                                             |
| `050-redis.tf`          | In-cluster Redis (`redis:7-alpine`) as JuiceFS metadata engine                      |
| `055-juicefs.tf`        | JuiceFS CSI driver (Helm), `juicefs-secret`, `juicefs-sc` StorageClass              |
| `070-demo-workload.tf`  | `juicefs-demo` namespace, shared PVC, 2-replica Deployment writing to the same file |

## Usage

```bash
cp prod.auto.tfvars.example prod.auto.tfvars
# set stackit_project_id and stackit_service_account_key_path

terraform init && terraform apply
```

## Verify: shared volume

```bash
terraform output -raw kubeconfig > kubeconfig.yaml
export KUBECONFIG=$(pwd)/kubeconfig.yaml

kubectl rollout status deployment/juicefs-demo -n juicefs-demo
kubectl exec -n juicefs-demo deployment/juicefs-demo -- tail -20 /data/shared.txt
```

Both replicas write to the same `/data/shared.txt` — lines from different pod hostnames interleaved.

## Verify: objects in S3

```bash
export AWS_ACCESS_KEY_ID=$(terraform output -raw juicefs_s3_access_key)
export AWS_SECRET_ACCESS_KEY=$(terraform output -raw juicefs_s3_secret_key)
export S3_ENDPOINT=$(terraform output -raw juicefs_s3_endpoint)
export BUCKET=$(terraform output -raw juicefs_bucket_name)

aws s3 ls s3://${BUCKET}/ --endpoint-url ${S3_ENDPOINT} --recursive --human-readable
```

## Gardener NetworkPolicy

SKE (Gardener) enforces a default-deny `NetworkPolicy` in `kube-system`. Pods must carry specific labels to open egress paths:

| Label                                                   | Opens                            |
| ------------------------------------------------------- | -------------------------------- |
| `networking.gardener.cloud/to-apiserver: allowed`       | Kubernetes API                   |
| `networking.gardener.cloud/to-dns: allowed`             | CoreDNS                          |
| `networking.gardener.cloud/to-public-networks: allowed` | Internet (S3, external services) |

Two sets of pods need these labels:

**CSI driver pods** (controller, node, dashboard): set via `controller.labels`, `node.labels`, `dashboard.labels` in the Helm values.

**Mount pods**: spawned dynamically in `kube-system` when a PVC is first mounted. They are not part of the Helm release and do not inherit the CSI driver labels. Patched via `globalConfig.mountPodPatch` (no `pvcSelector` = all mount pods). Without this, DNS resolution fails with `i/o timeout`.

## Production notes

The in-cluster Redis in `050-redis.tf` is for demo purposes only. It has no persistence guarantees and is a single point of failure for all JuiceFS volumes. For production, replace the `metaurl` in `055-juicefs.tf` with a managed service:

- Managed Redis or Valkey on STACKIT
- Self-hosted Redis with replication and AOF persistence
