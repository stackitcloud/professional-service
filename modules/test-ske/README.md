<!-- tags: ske, kubernetes, test, cluster -->

# Test SKE Module

This module is designed to quickly spin up an SKE cluster. Internally, we use it to
debug network connectivity and deploy test applications in a simple, frictionless manner.
It automatically selects the latest SKE and node pool machine versions.

<!-- BEGIN_TF_DOCS -->

## Requirements

| Name                                                               | Version  |
| ------------------------------------------------------------------ | -------- |
| <a name="requirement_random"></a> [random](#requirement_random)    | 3.9.0    |
| <a name="requirement_stackit"></a> [stackit](#requirement_stackit) | >=0.95.0 |

## Providers

| Name                                                         | Version  |
| ------------------------------------------------------------ | -------- |
| <a name="provider_random"></a> [random](#provider_random)    | 3.9.0    |
| <a name="provider_stackit"></a> [stackit](#provider_stackit) | >=0.95.0 |

## Modules

No modules.

## Resources

| Name                                                                                                                                                        | Type        |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| [random_string.this](https://registry.terraform.io/providers/hashicorp/random/3.9.0/docs/resources/string)                                                  | resource    |
| [stackit_ske_cluster.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/ske_cluster)                                  | resource    |
| [stackit_ske_kubeconfig.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/resources/ske_kubeconfig)                            | resource    |
| [stackit_ske_kubernetes_versions.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/data-sources/ske_kubernetes_versions)       | data source |
| [stackit_ske_machine_image_versions.this](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs/data-sources/ske_machine_image_versions) | data source |

## Inputs

| Name                                                                  | Description                                                                                                                                                                                                  | Type                                                                                                                                                             | Default                                                                                                                                                                                                                                                                                                                            | Required |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------: |
| <a name="input_cluster_name"></a> [cluster_name](#input_cluster_name) | The name of the Kubernetes cluster                                                                                                                                                                           | `string`                                                                                                                                                         | `null`                                                                                                                                                                                                                                                                                                                             |    no    |
| <a name="input_extensions"></a> [extensions](#input_extensions)       | SKE cluster extensions (e.g. application_load_balancer, acl, dns). Passed through directly to the stackit_ske_cluster resource.                                                                              | `any`                                                                                                                                                            | `null`                                                                                                                                                                                                                                                                                                                             |    no    |
| <a name="input_maintenance"></a> [maintenance](#input_maintenance)    | Maintenance window configuration for the cluster                                                                                                                                                             | <pre>object({<br/> enable_kubernetes_version_updates = bool<br/> enable_machine_image_version_updates = bool<br/> start = string<br/> end = string<br/> })</pre> | <pre>{<br/> "enable_kubernetes_version_updates": true,<br/> "enable_machine_image_version_updates": true,<br/> "end": "05:00:00Z",<br/> "start": "04:00:00Z"<br/>}</pre>                                                                                                                                                           |    no    |
| <a name="input_network_id"></a> [network_id](#input_network_id)       | The ID of the STACKIT network in which the SKE cluster will be deployed. If not provided, the cluster will automatically create a network on demand. Specifying a network ID is only supported in SNA setups | `string`                                                                                                                                                         | `null`                                                                                                                                                                                                                                                                                                                             |    no    |
| <a name="input_node_pools"></a> [node_pools](#input_node_pools)       | Configuration for the cluster node pools                                                                                                                                                                     | `any`                                                                                                                                                            | <pre>[<br/> {<br/> "availability_zones": [<br/> "eu01-1",<br/> "eu01-2",<br/> "eu01-3"<br/> ],<br/> "machine_type": "g2i.4",<br/> "max_surge": 3,<br/> "maximum": 3,<br/> "minimum": 1,<br/> "name": "standard",<br/> "os_name": "flatcar",<br/> "volume_size": 20,<br/> "volume_type": "storage_premium_perf6"<br/> }<br/>]</pre> |    no    |
| <a name="input_project_id"></a> [project_id](#input_project_id)       | The STACKIT project ID                                                                                                                                                                                       | `string`                                                                                                                                                         | n/a                                                                                                                                                                                                                                                                                                                                |   yes    |

## Outputs

| Name                                                                    | Description                                   |
| ----------------------------------------------------------------------- | --------------------------------------------- |
| <a name="output_cluster_name"></a> [cluster_name](#output_cluster_name) | The name of the provisioned SKE cluster       |
| <a name="output_kubeconfig"></a> [kubeconfig](#output_kubeconfig)       | The kubeconfig contents to access the cluster |
