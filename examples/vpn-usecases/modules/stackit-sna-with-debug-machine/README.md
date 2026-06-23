# SNA with test-machine module

This module is used to quickly spin up a sna with a test virtual machine. We use this module to debug vpn connectivity.

> ⚠️ **SECURITY WARNING**
> Be careful: By default, **port security is disabled** on the network interface to allow unrestricted traffic for debugging purposes. **Do not use this module in a production environment**.

<!-- BEGIN_TF_DOCS -->

## Requirements

| Name                                                               | Version  |
| ------------------------------------------------------------------ | -------- |
| <a name="requirement_stackit"></a> [stackit](#requirement_stackit) | >=0.95.0 |

## Providers

| Name                                                         | Version  |
| ------------------------------------------------------------ | -------- |
| <a name="provider_stackit"></a> [stackit](#provider_stackit) | >=0.95.0 |

## Modules

No modules.

## Resources

| Name                                                                                                                                               | Type     |
| -------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| [stackit_network.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/network)                                 | resource |
| [stackit_network_area.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/network_area)                       | resource |
| [stackit_network_area_region.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/network_area_region)         | resource |
| [stackit_network_interface.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/network_interface)             | resource |
| [stackit_public_ip.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/public_ip)                             | resource |
| [stackit_resourcemanager_project.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/resourcemanager_project) | resource |
| [stackit_server.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/server)                                   | resource |
| [stackit_volume.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/volume)                                   | resource |

## Inputs

| Name                                                                                                                        | Description                                                                                                                  | Type           | Default                                  | Required |
| --------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | -------------- | ---------------------------------------- | :------: |
| <a name="input_machine_availability_zone"></a> [machine_availability_zone](#input_machine_availability_zone)                | The availability zone (e.g. eu01-1)                                                                                          | `string`       | n/a                                      |   yes    |
| <a name="input_machine_disk_performance_class"></a> [machine_disk_performance_class](#input_machine_disk_performance_class) | Storage performance class                                                                                                    | `string`       | `"storage_premium_perf4"`                |    no    |
| <a name="input_machine_disk_size"></a> [machine_disk_size](#input_machine_disk_size)                                        | Boot volume size in GB                                                                                                       | `number`       | `20`                                     |    no    |
| <a name="input_machine_image_id"></a> [machine_image_id](#input_machine_image_id)                                           | Image UUID (Default: Debian 12)                                                                                              | `string`       | `"c751cde7-e648-4f81-9722-ce9c7848bed0"` |    no    |
| <a name="input_machine_ipv4_prefix"></a> [machine_ipv4_prefix](#input_machine_ipv4_prefix)                                  | The IPv4 prefix for the test machine's network (CIDR notation). This must be a subnet within the defined SNA network ranges. | `string`       | n/a                                      |   yes    |
| <a name="input_machine_name"></a> [machine_name](#input_machine_name)                                                       | name of the stackit test machine                                                                                             | `string`       | n/a                                      |   yes    |
| <a name="input_machine_network_name"></a> [machine_network_name](#input_machine_network_name)                               | The name of the network where the test machine will be connected.                                                            | `string`       | n/a                                      |   yes    |
| <a name="input_machine_type"></a> [machine_type](#input_machine_type)                                                       | Flavor of the machine                                                                                                        | `string`       | `"c2i.1"`                                |    no    |
| <a name="input_sna_default_nameserver"></a> [sna_default_nameserver](#input_sna_default_nameserver)                         | A list of STACKIT SNA default nameservers (IP addresses).                                                                    | `list(string)` | <pre>[<br/> "1.1.1.1"<br/>]</pre>        |    no    |
| <a name="input_sna_name"></a> [sna_name](#input_sna_name)                                                                   | The name of the STACKIT Network Area (SNA).                                                                                  | `string`       | n/a                                      |   yes    |
| <a name="input_sna_network_range_prefix"></a> [sna_network_range_prefix](#input_sna_network_range_prefix)                   | A list of STACKIT SNA network range prefixes in CIDR notation.                                                               | `list(string)` | <pre>[<br/> "10.28.0.0/16"<br/>]</pre>   |    no    |
| <a name="input_sna_transfer_range"></a> [sna_transfer_range](#input_sna_transfer_range)                                     | The STACKIT SNA transfer range in CIDR notation.                                                                             | `string`       | `"172.16.0.0/16"`                        |    no    |
| <a name="input_stackit_admin_email"></a> [stackit_admin_email](#input_stackit_admin_email)                                  | The email address of the project administrator.                                                                              | `string`       | n/a                                      |   yes    |
| <a name="input_stackit_org_id"></a> [stackit_org_id](#input_stackit_org_id)                                                 | The STACKIT Organization ID (UUID).                                                                                          | `string`       | n/a                                      |   yes    |
| <a name="input_stackit_project_name"></a> [stackit_project_name](#input_stackit_project_name)                               | The name of the STACKIT project where the managed VPN and test machine will be deployed.                                     | `string`       | n/a                                      |   yes    |

## Outputs

| Name                                                                                            | Description                                                |
| ----------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| <a name="output_machine_network_ipv4"></a> [machine_network_ipv4](#output_machine_network_ipv4) | The IPv4 prefix of the machine's network.                  |
| <a name="output_machine_private_ipv4"></a> [machine_private_ipv4](#output_machine_private_ipv4) | The private IP address of the test machine.                |
| <a name="output_machine_public_ip"></a> [machine_public_ip](#output_machine_public_ip)          | The public IP address of the test machine.                 |
| <a name="output_project_id"></a> [project_id](#output_project_id)                               | The ID of the STACKIT project.                             |
| <a name="output_sna_id"></a> [sna_id](#output_sna_id)                                           | The ID of the STACKIT Network Area.                        |
| <a name="output_sna_network_range"></a> [sna_network_range](#output_sna_network_range)          | The network ranges (sna-ipv4) of the STACKIT Network Area. |

<!-- END_TF_DOCS -->
