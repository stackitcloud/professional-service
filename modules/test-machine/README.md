# Test Machine Module

This module is used to quickly spin up a test virtual machine. Internally, we use this module to debug network connectivity and cloud-init configurations in a fast, frictionless manner.

> ⚠️ **SECURITY WARNING**
> Be careful: By default, **port security is disabled** (`security_enabled = false`) on the network interface to allow unrestricted traffic for debugging purposes. **Do not use this module in a production environment** without explicitly setting `security_enabled = true` and applying proper security group rules.

## Usage

```terraform
module "test-machine" {
  source = "../modules/test-machine"

  project_id        = stackit_resourcemanager_project.this.project_id
  network_id        = stackit_network.this.network_id
  availability_zone = var.default_zone

  name         = "vm-debug"
  machine_type = var.jumphost_flavor
  disk_size    = 500

  user_data = templatefile("${path.module}/../cloud-init/vm-debug.yaml", {})
}
```

## Inputs

| Name                     | Description                                               | Type     | Default                                  | Required |
| ------------------------ | --------------------------------------------------------- | -------- | ---------------------------------------- | :------: |
| `project_id`             | The STACKIT Project ID                                    | `string` | n/a                                      | **yes**  |
| `network_id`             | The Network ID (UUID) where the machine should be spawned | `string` | n/a                                      | **yes**  |
| `availability_zone`      | The availability zone (e.g. eu01-1)                       | `string` | n/a                                      | **yes**  |
| `name`                   | Hostname of the server                                    | `string` | `"test-machine"`                         |    no    |
| `machine_type`           | Flavor of the machine                                     | `string` | `"g1.1"`                                 |    no    |
| `image_id`               | Image UUID (Default: Debian 12)                           | `string` | `"c751cde7-e648-4f81-9722-ce9c7848bed0"` |    no    |
| `disk_size`              | Boot volume size in GB                                    | `number` | `50`                                     |    no    |
| `disk_performance_class` | Storage performance class                                 | `string` | `"storage_premium_perf4"`                |    no    |
| `user_data`              | Cloud-init configuration string                           | `string` | `""`                                     |    no    |
| `security_enabled`       | Enable security (port security) on the network interface  | `bool`   | `false`                                  |    no    |

## Outputs

| Name          | Description                                                 |
| ------------- | ----------------------------------------------------------- |
| `server_id`   |                                                             |
| `server_name` |                                                             |
| `primary_ip`  | The primary ipv4 internal address assigned to the interface |
