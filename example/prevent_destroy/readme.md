## Prevent Destroy for Terraform Resources

This example configuration demonstrates how to use the `mapotf` tool to control the `prevent_destroy` lifecycle setting of Terraform resources. This is achieved by using the `update_in_place` transform block. Here's a step-by-step explanation of how it works:

### Purpose

The primary purpose of the `prevent_destroy` configuration is to:

- **Prevent Accidental Destruction**: Ensure that critical resources are not accidentally destroyed by setting the `prevent_destroy` lifecycle setting.
- **Control Resource Lifecycle**: Provide a mechanism to control the lifecycle of resources dynamically based on configuration variables.
- **Flexibility**: Allow the configuration to be applied only to the root module or to all modules based on the `root_only` variable.

### Configuration

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

Since we are using the `asstring` attribute, `mapotf` would try to evaluate the value of `var.prevent_destroy` in the mapotf context. Let's say the value is `false` in `bool` type, which could be converted to `false` in `string` type automatically, then the generated patch block would be like:

```hcl
lifecycle {
  prevent_destroy = false
}
```

You can also use the `dynamic_block_body` attribute to achieve the same result:

```terraform
transform "update_in_place" set_prevent_destroy {
  for_each             = try(local.addresses, [])
  target_block_address = each.value
  dynamic_block_body = <<-DYNAMIC_BODY
    lifecycle {
      prevent_destroy = ${var.prevent_destroy}
    }
DYNAMIC_BODY
}
```

### Example

Before running this example, you would see the `main.tf` file like this:

```terraform
resource "random_id" "rg_name" {
  byte_length = 8
}

resource "azurerm_resource_group" "example" {
  location = var.location
  name     = "azure-subnets-${random_id.rg_name.hex}-rg"
}

locals {
  subnets = {
    for i in range(3) : "subnet${i}" => {
      address_prefixes = [cidrsubnet(local.virtual_network_address_space, 8, i)]
    }
  }
  virtual_network_address_space = "10.0.0.0/16"
}

module "vnet" {
  source                        = "Azure/subnets/azurerm"
  version                       = "1.0.0"
  resource_group_name           = azurerm_resource_group.example.name
  subnets                       = local.subnets
  virtual_network_address_space = [local.virtual_network_address_space]
  virtual_network_location      = var.vnet_location
  virtual_network_name          = "azure-subnets-vnet"
}
```

To run the demo, you must run `terraform init` or `mapotf init` first. After `init`, the module `Azure/subnets/azurerm`'s source code would be downloaded to the `.terraform/modules` directory.

After running `mapotf transform -r --mptf-dir . --tf-dir .`, the `prevent_destroy` lifecycle setting would be applied to the resources based on the configuration. You can check configs under `.terraform/modules` to see the changes, all resource blocks declared in the referenced module would also be updated.

Or, you can run `mapotf apply -r --mptf-dir . --tf-dir .`. `mapotf` would apply the transformation and then run `terraform apply` to apply the changes to the Terraform configuration. No matter how Terraform ends up applying the changes, after apply all transformations made by `mapotf` would be reverted. These transformations only exist while Terraform is running.

In summary, this example demonstrates how to use the `mapotf` tool to control the `prevent_destroy` lifecycle setting of Terraform resources. The `update_in_place` transform block is a powerful tool that allows you to modify existing blocks in place, making it possible to add or modify lifecycle settings without recreating resources.