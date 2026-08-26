<!-- tags: ske, nfs, sfs, storage, kubernetes, rwx, file-storage -->

# STACKIT File Storage Example Deployment

Terraform Example of deploying a STACKIT File Storage NFS Service

## Deployment Scope

- Network Area with Routing Tables Enabled
- Projects attached to the Network area
- STACKIT SFS Resources
- SKE Cluster for RWX usage

## Setup RWX on SKE with STACKIT SFS

**Install Helmchart**

```bash
helm repo add nfs-subdir-external-provisioner https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/
helm repo update
```

```bash
helm install nfs-provisioner nfs-subdir-external-provisioner/nfs-subdir-external-provisioner \
    --set nfs.server=1.2.3.4 \
    --set nfs.path=/srv/nfs/storage \
    --set storageClass.name=nfs-client
```

**Create PersistentVolumeClaim from NFS Storage**

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: test-claim
spec:
  storageClassName: nfs-client
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Mi
```
