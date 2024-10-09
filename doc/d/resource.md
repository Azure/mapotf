# Data "Resource" Block

The `data "resource"` block is used to query and retrieve `resource` blocks from a Terraform configuration. This block allows you to filter and collect resource blocks based on specific criteria such as the resource type, and whether `count` or `for_each` is used.

## Arguments

- `resource_type`: This optional argument specifies the type of the resource to filter. It is a string attribute.
- `use_count`: This optional argument is a boolean that, when set to `true`, filters resource blocks that use the `count` attribute. The default value is `false`.
- `use_for_each`: This optional argument is a boolean that, when set to `true`, filters resource blocks that use the `for_each` attribute. The default value is `false`.

## Attributes

- `result`: This attribute contains the filtered resource blocks, all expressions assigned to arguments are evaluated as strings.

## Example - Querying Resource Blocks

Here is an example of how to use the `data "resource"` block to query resource blocks of a specific type:

```terraform
data "resource" "aks" {
  resource_type = "azurerm_kubernetes_cluster"
}
```

In this example, the `data "resource"` block queries all resource blocks of type `azurerm_kubernetes_cluster` and stores the result in the `example` data source.

The following Mapotf expressions could help you to access the results:

```terraform
locals {
  kubernetes_cluster_resource_blocks     = flatten([for _, blocks in flatten(data.resource.all.result) : [for b in blocks : b]])
  kubernetes_cluster_resource_blocks_map = { for block in local.kubernetes_cluster_resource_blocks : block.mptf.block_address => block }
}
```

Assuming we have the following Terraform configuration:

```terraform
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"

  default_node_pool {
    name       = "default"
    node_count = 1
    vm_size    = "Standard_D2_v2"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = {
    Environment = "Production"
  }
}
```

In Mapotf, when you're referring `local.kubernetes_cluster_resource_blocks`, the result would be a `list(object)` that contains one element that corresponding to `resource.azurerm_kubernetes_cluster.example` block in Terraform config, and `local.kubernetes_cluster_resource_blocks_map` converted this list into a `map(object)`, with one element and `"resource.azurerm_kubernetes_cluster.example"` as the key.

## Example - Filtering Resource Blocks with `count`

Here is an example of how to use the `data "resource"` block to filter resource blocks that use the `count` attribute:

```terraform
data "resource" "example" {
  resource_type = "fake_resource"
  use_count     = true
}
```

In this example, the `data "resource"` block filters resource blocks of type `fake_resource` that use the `count` attribute and stores the result in the `example` data source.

## Example - Filtering Resource Blocks with `for_each`

Here is an example of how to use the `data "resource"` block to filter resource blocks that use the `for_each` attribute:

```terraform
data "resource" "example" {
  resource_type = "fake_resource"
  use_for_each  = true
}
```

In this example, the `data "resource"` block filters resource blocks of type `fake_resource` that use the `for_each` attribute and stores the result in the `example` data source.

The data source's results would be aggregated by resource type first, then by the block labels.