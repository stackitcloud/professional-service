<!-- tags: iaas, compute, test, vm, cloud-init -->

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

## Requirements

| Name                                                               | Version   |
| ------------------------------------------------------------------ | --------- |
| <a name="requirement_stackit"></a> [stackit](#requirement_stackit) | >= 0.87.0 |

## Providers

| Name                                                         | Version   |
| ------------------------------------------------------------ | --------- |
| <a name="provider_stackit"></a> [stackit](#provider_stackit) | >= 0.87.0 |

## Modules

No modules.

## Resources

| Name                                                                                                                                  | Type     |
| ------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| [stackit_network_interface.nic](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/network_interface) | resource |
| [stackit_server.server](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/server)                    | resource |
| [stackit_volume.boot_volume](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/volume)               | resource |

## Inputs

| Name                                                                                                | Description                                               | Type     | Default                                  | Required |
| --------------------------------------------------------------------------------------------------- | --------------------------------------------------------- | -------- | ---------------------------------------- | :------: |
| <a name="input_availability_zone"></a> [availability_zone](#input_availability_zone)                | The availability zone (e.g. eu01-1)                       | `string` | n/a                                      |   yes    |
| <a name="input_disk_performance_class"></a> [disk_performance_class](#input_disk_performance_class) | Storage performance class                                 | `string` | `"storage_premium_perf4"`                |    no    |
| <a name="input_disk_size"></a> [disk_size](#input_disk_size)                                        | Boot volume size in GB                                    | `number` | `50`                                     |    no    |
| <a name="input_image_id"></a> [image_id](#input_image_id)                                           | Image UUID (Default: Debian 12)                           | `string` | `"c751cde7-e648-4f81-9722-ce9c7848bed0"` |    no    |
| <a name="input_machine_type"></a> [machine_type](#input_machine_type)                               | Flavor of the machine                                     | `string` | `"g1.1"`                                 |    no    |
| <a name="input_name"></a> [name](#input_name)                                                       | Hostname of the server                                    | `string` | `"test-machine"`                         |    no    |
| <a name="input_network_id"></a> [network_id](#input_network_id)                                     | The Network ID (UUID) where the machine should be spawned | `string` | n/a                                      |   yes    |
| <a name="input_project_id"></a> [project_id](#input_project_id)                                     | The STACKIT Project ID                                    | `string` | n/a                                      |   yes    |
| <a name="input_security_enabled"></a> [security_enabled](#input_security_enabled)                   | Enable security (port security) on the network interface  | `bool`   | `false`                                  |    no    |
| <a name="input_user_data"></a> [user_data](#input_user_data)                                        | Cloud-init configuration string                           | `string` | `""`                                     |    no    |

## Outputs

| Name                                                                 | Description                                                 |
| -------------------------------------------------------------------- | ----------------------------------------------------------- |
| <a name="output_primary_ip"></a> [primary_ip](#output_primary_ip)    | The primary ipv4 internal address assigned to the interface |
| <a name="output_server_id"></a> [server_id](#output_server_id)       | n/a                                                         |
| <a name="output_server_name"></a> [server_name](#output_server_name) | n/a                                                         |
