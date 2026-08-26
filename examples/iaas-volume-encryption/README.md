<!-- tags: iaas, volume, encryption, kms, block-storage -->

# IaaS Volume Encryption (Terraform)

## Terraform Examples

KMS & IaaS Resources to deploy a encrypted Block Storage Volume

### Migrations Steps to move data von non encrypted Volumes to a encrypted Volume

1. Create Backup of non encrypted Volume

There are two options to perform a Backup the first one is to reference a volume directly:

```bash
stackit volume backup create --source-type volume --source-id <volumeId> --name backup01
```

> This does block all operations on the Volume such as extending the Volume until the backup is done.

Another Option is to first create a Snapshot and make a Backup von this Snapshot:

```bash
stackit volume backup create --source-type snapshot --source-id <snapshotId> --name backup01
```

2. Create new encrypted Volume
   Use the provided Terraform to deploy a new encrypted Volume with the same size (or larger) then the original Volume.

3. Create new encrypted Volume from Backup
   Use the Backup as a source for a new encrypted Volume.

```hcl
resource "stackit_volume" "encrypted" {
  project_id        = var.STACKIT_PROJECT_ID
  name              = "encrypted_volume"
  availability_zone = var.zone
  performance_class = "storage_premium_perf6"
  size              = 4
  source = {
    type = "backup"
    id = "<backupId>"
  }
  encryption_parameters = {
    kek_key_id = stackit_kms_key.volume-key.key_id
    kek_key_version = 1
    kek_keyring_id = stackit_kms_keyring.volume.keyring_id
    service_account = "<serviceAccount>@sa.stackit.cloud"
  }
}
```

4. Recreate VM or attach volume to existing VM

```bash
stackit server create --availability-zone eu01-3 --machine-type c2i.2 --boot-volume-source-type volume --boot-volume-source-id <volumeId> --network-id <networkId> -n server1
```
