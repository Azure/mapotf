# Update In Place Transform Block

The `update_in_place` transform block is a powerful tool in Terraform that allows you to modify existing blocks in place. This is particularly useful when you want to add or modify attributes of a resource without recreating it.

## Arguments

- `target_block_address`: This argument is used to specify the address of the block that the transformation will be applied to. The block address is a string that uniquely identifies a block in a Terraform configuration.

- `asstring`: This attribute is used to specify the transformation that will be applied to the resources. The transformation is defined as a string of Terraform code.

- `asraw`: This attribute is used to specify the transformation that will be applied to the resources. The transformation is defined as raw HCL code. The code is not parsed or evaluated, but is directly inserted into the Terraform configuration. This allows you to write complex transformations that cannot be expressed as a single Terraform expression.

## Example

Here is an example of how to use the `update_in_place` transform block to add tags to Azure Kubernetes Cluster resources:

```terraform
data "resource" azurerm_kubernetes_cluster {
  resource_type = "azurerm_kubernetes_cluster"
}

transform "update_in_place" tracing_tags {
  for_each             = try(data.resource.azurerm_kubernetes_cluster.result.azurerm_kubernetes_cluster, {})
  target_block_address = each.value.mptf.block_address
  asstring {
    tags = <<-TAGS
      merge({
        file = "${each.value.mptf.range.file_name}"
        block = "${each.value.mptf.terraform_address}"
        git_hash = "${each.value.mptf.module.git_hash}"
        module_source = "${each.value.mptf.module.source}"
        module_version = "${each.value.mptf.module.version}"
      }, ${try(each.value.tags, "{}")})
TAGS
  }
}
```

In this example, the `for_each` argument is set to the result of the `azurerm_kubernetes_cluster` data source. The `target_block_address` argument is set to the block address of each resource in the collection. The `asstring` attribute is used to define a transformation that merges a set of new tags with the existing tags of each resource. The new tags include the file name, block address, git hash, module source, and module version of the resource.

In the [`prevent_destroy\main.mptf.hcl`](../example/prevent_destroy/main.mptf.hcl) example, the `mapotf` tool is used to control the `prevent_destroy` lifecycle setting of Terraform resources. This is achieved by using the `update_in_place` transform block. Here's a step-by-step explanation of how it works:

1. **Define Variables**: Two variables are defined at the beginning of the file. The `prevent_destroy` variable is a boolean that determines whether to prevent the destruction of resources. The `root_only` variable is also a boolean that determines whether to apply the transformation only to the root module.

```terraform
variable "prevent_destroy" {
  type    = bool
  default = false
}

variable "root_only" {
  type    = bool
  default = false
}
```

2. **Fetch All Resources**: The `data "resource" all_resource` block is used to fetch all resources in the Terraform configuration. This data block does not require a `resource_type` attribute, as it fetches all resources regardless of their type.

```terraform
data "resource" all_resource {
}
```

3. **Prepare Resource Addresses**: The `locals` block is used to prepare a list of block addresses for all resources. If the `root_only` variable is set to `true`, only the block addresses of resources in the root module are included in the list. Otherwise, the block addresses of all resources are included.

```terraform
locals {
  all_resource_blocks = flatten([
    for resource_type, resource_blocks in data.resource.all_resource.result : resource_blocks
  ])
  mptfs     = flatten([for _, blocks in local.all_resource_blocks : [for b in blocks : b.mptf]])
  addresses = var.root_only ? [for mptf in local.mptfs : mptf.block_address if mptf.module.dir == "."] : [for mptf in local.mptfs : mptf.block_address]
}
```

4. **Apply Transform**: The `update_in_place` transform block is used to update the `prevent_destroy` lifecycle setting of each resource. The `for_each` argument is set to the list of block addresses prepared in the previous step. The `target_block_address` argument is set to the block address of each resource. The `asstring` attribute is used to define a transformation that sets the `prevent_destroy` lifecycle setting to the value of the `prevent_destroy` variable.

```terraform
transform "update_in_place" set_prevent_destroy {
  for_each             = try(local.addresses, [])
  target_block_address = each.value

  asstring {
    lifecycle {
      prevent_destroy = var.prevent_destroy
    }
  }
}
```

In summary, this example demonstrates how to use the `mapotf` tool to control the `prevent_destroy` lifecycle setting of Terraform resources. The `update_in_place` transform block is a powerful tool that allows you to modify existing blocks in place, making it possible to add or modify lifecycle settings without recreating resources.
