<!-- tags: ske, nfs, sfs, storage, kubernetes, rwx, file-storage, csi, ephemeral -->

# STACKIT File Storage on SKE

Mounts a STACKIT File Storage share as a `ReadWriteMany` volume in an SKE cluster, so
several pods on different nodes write to the same store.

| Stage                      | Creates                                                                  |
| -------------------------- | ------------------------------------------------------------------------ |
| [`01-storage`](01-storage) | Network area, project, SFS resource pool, export policy and share        |
| [`02-cluster`](02-cluster) | Network, SKE cluster, `csi-driver-nfs` and the `nfs-client` StorageClass |

Two stages, because the resource pool is slow and only `01-storage` needs rights on the
organization.

## Order

```bash
cd 01-storage
cp terraform.tfvars.example terraform.tfvars   # org, parent container, email
terraform init && terraform apply

cd ../02-cluster
terraform init && terraform apply
```

`02-cluster` reads the project ID and the mount path out of
`../01-storage/terraform.tfstate`. Nothing is copied by hand.

Tear down in reverse.

## Requirements

- Terraform 1.5 or later, 1.10 for `02-cluster` because of its ephemeral resource.
- STACKIT provider 0.116.0 or later.
- Rights on the organization for `01-storage`. It creates the network area, and the
  project that carries the cluster: SKE rejects a cluster whose project sits in a folder.
- A service account key for Terraform, see `stackit_service_account_key_path` in the
  stages' `020-variables.tf`.
- An authenticated `stackit` CLI (`stackit auth login` or
  `stackit auth activate-service-account`) for the timeout check of `01-storage` and the
  verify step of `02-cluster`.
- `kubectl` for the verify step of `02-cluster`.

`enable_beta_resources` and `experiments = ["ske"]` are already set where needed.

## How the two halves meet

The share's `mount_path` has the form `10.2.1.1:/rp_VKL20Ub/nfs-share`. `02-cluster`
splits it into the StorageClass parameters `server` and `share`. The nodes reach the
share because the cluster network takes its prefix from the network area range, which is
what the export policy accepts.
