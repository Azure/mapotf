## Applying Default Tags with `default_tags` Configuration

This example configuration is designed to ensure that all Terraform-managed AzureRM resources within a project that support tagging are automatically tagged with a predefined set of tags. This is particularly useful for maintaining consistency, facilitating resource management, and adhering to organizational policies regarding resource tagging.

### Purpose

The primary purpose of the `default_tags` configuration is to:

- **Automate Tagging**: Automatically apply a default set of tags (`hello = "world"`) to all resources that support tagging, without the need to manually specify these tags for each resource.
- **Ensure Consistency**: Help maintain a consistent tagging strategy across your infrastructure, which is crucial for resource organization, cost tracking, and access control.
- **Simplify Management**: By applying tags automatically, it simplifies the management of resources, especially in large-scale environments where manual tagging can be error-prone and time-consuming.

This configuration leverages the `mapotf` tool's capability to dynamically modify Terraform code, making it easier to enforce tagging policies across multiple resources and projects.

Before running this example, you would see [`main.tf`](./main.tf) file like this:

```hcl
resource "azurerm_resource_group" "this" {
  location = "West US"
  name     = "example-resources"
}

resource "azurerm_storage_account" "this" {
  name                     = "storageaccountname"
  resource_group_name      = azurerm_resource_group.this.name
  location                 = azurerm_resource_group.this.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags = {
    env = "prod"
  }
}

resource "azurerm_subnet" "this" {
  address_prefixes = []
  name                 = ""
  resource_group_name  = ""
  virtual_network_name = ""
}
```

You can run `mapotf transform --mptf-dir . --tf-dir .`, then you would see:

```hcl
resource "azurerm_resource_group" "this" {
  location = "West US"
  name     = "example-resources"
  tags = {
    file           = "main.tf"
    block          = "azurerm_resource_group.this"
    module_source  = try(one(data.modtm_module_source.telemetry).module_source, "")
    module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
  }

}

resource "azurerm_storage_account" "this" {
  name                     = "storageaccountname"
  resource_group_name      = azurerm_resource_group.this.name
  location                 = azurerm_resource_group.this.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags = merge({
    env = "prod"
    }, {
    file           = "main.tf"
    block          = "azurerm_storage_account.this"
    module_source  = try(one(data.modtm_module_source.telemetry).module_source, "")
    module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
  })

}

resource "azurerm_subnet" "this" {
  address_prefixes     = []
  name                 = ""
  resource_group_name  = ""
  virtual_network_name = ""
}
```

`azurerm_resource_group` and `azurerm_storage_account` supports `tags` so a default tags has been added. `azurerm_subnet` doesn't has tags, so no changes.

When `mapotf` applied default tags, the original tags on `azurerm_storage_account.this` would be honored by using `merge` function.

## Under the hood

```hcl
data "resource" all {
}
```

This data source block would retrieve all `resource` blocks in current Terraform configs.

```hcl
data "terraform" version {
}
```

This data source block would retrieve `terraform` block in current Terraform configs.

```hcl
data "provider_schema" azurerm {
  provider_source = data.terraform.version.required_providers["azurerm"].source
  provider_version = data.terraform.version.required_providers["azurerm"].version
}
```

This `provider_schema` data source takes in two arguments: 

* `provider_source`: Corresponding to [`source`](https://developer.hashicorp.com/terraform/language/providers/requirements#source) argument in Terraform [`required_providers`](https://developer.hashicorp.com/terraform/language/providers/requirements) block
* `provider_version`: Corresponding to [`version`](https://developer.hashicorp.com/terraform/language/providers/requirements#source) argument in Terraform [`required_providers`](https://developer.hashicorp.com/terraform/language/providers/requirements) block

When Mapotf evaluates this data source, it would generate such a temporary Terraform config in temp dir:

```hcl
terraform {
  required_providers {
    provider = {
      source  = "<SOURCE_FROM_MAPOTF_CONFIG>"
      version = "<VERSION_FROM_MAPOTF_CONFIG>"
    }
  }
}
```

Then run `terraform init` and `terraform providers schema -json` against it, return the response as `*tfjson.Schema` type.

`data.terraform.version.required_providers["azurerm"].source` would retrieve `azurerm` provider's `source` defined in current Terraform config, so we won't need to hard code one. We can always use the version defined by current Terraform config.

```hcl
resources_support_tags = toset([ for name, r in data.provider_schema.azurerm.resources : name if try(r.block.attributes["tags"].type == ["map", "string"], false) ])
```

This expression filters out resource type that contains an attribute named `tags`, and with type `["map", "string"]`. In the schema returned by `terraform providers schema -json`, composite type like `map(string)`'s denotation is `["map", "string"]`. You can just check Terraform's output, copy and paste the type directly in Mapotf configs.

```hcl
resource_support_tags = flatten([ for resource_type, resource_blocks in data.resource.all.result : resource_blocks if contains(local.resources_support_tags, resource_type) ])
```

This expression matchs all `resource` blocks defined in current Terraform config with resource type that contains `tags` we want.

```hcl
mptfs = flatten([for _, blocks in local.resource_support_tags : [for b in blocks : b.mptf]])
```

Every Terraform resource block retrieved by Mapotf data source contains an object named `mptf`, which contains:

* `block_address`: Full path to the Terraform block, like `resource.azurerm_resource_group.this`
* `terraform_address`: Terraform address to the corresponding Terraform block, like `azurerm_resource_group.this`
* `module`: An object contains the following module metadata defined in the `modules.json` file we could retrieve:
  - `key`: `Key` in `modules.json` file
  - `version`: `Version` in `modules.json` file
  - `source`: `Source` in `modules.json` file
  - `dir`: `Dir` in `modules.json` file
  - `abs_dir`: The absolute path of `Dir`
  - `git_hash`: If the module is hosted by git even it was retrieved from `registry.terraform.io`, we still could get its git hash. If no git hash is available, `git_hash` would be empty.
* `range`: An object contains matched Terraform block's range in Terraform config file:
  - `file_name`: The file that matched block belongs to
  - `start_line`: Start line of the block
  - `start_column`: Start column of the block
  - `end_line`: End line of the block
  - `end_column`: End column of the block

```hcl
addresses = [for mptf in local.mptfs : mptf.block_address]
```

This expression returns block addresses of the Terraform resource blocks that support `tags`.

```hcl
all_resources = { for obj in flatten([for obj in flatten([for b in data.resource.all.result.* : [for nb in b : nb]]) : [for body in obj : body]]) : obj.mptf.block_address=>obj}
```

This expression returns a map from all resource blocks' block addresses to the block it selves.

```hcl
transform "update_in_place" tags {
  for_each = try(local.addresses, [])
  target_block_address = each.value
  asstring {
    tags = <<-TAGS
    %{if try(local.all_resources[each.value].tags != "", false)}merge(${local.all_resources[each.value].tags}, var.tracing_tags_enabled ? {
  file = "${local.all_resources[each.value].mptf.range.file_name}"
  block = "${local.all_resources[each.value].mptf.terraform_address}"
  module_source = try(one(data.modtm_module_source.telemetry).module_source, "")
  module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
} : {}) %{else} var.tracing_tags_enabled ? {
  file = "${local.all_resources[each.value].mptf.range.file_name}"
  block = "${local.all_resources[each.value].mptf.terraform_address}"
  module_source = try(one(data.modtm_module_source.telemetry).module_source, "")
  module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
} : {}%{ endif}
TAGS
  }
}
```

```hcl
for_each = try(local.addresses, [])
```

For each matched resource blocks that support `tags` we would execute a transform.

```hcl
target_block_address = each.value
```

```hcl
tags = <<-TAGS
    %{if try(local.all_resources[each.value].tags != "", false)}merge(${local.all_resources[each.value].tags}, var.tracing_tags_enabled ? {
  file = "${local.all_resources[each.value].mptf.range.file_name}"
  block = "${local.all_resources[each.value].mptf.terraform_address}"
  module_source = try(one(data.modtm_module_source.telemetry).module_source, "")
  module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
} : {}) %{else} var.tracing_tags_enabled ? {
  file = "${local.all_resources[each.value].mptf.range.file_name}"
  block = "${local.all_resources[each.value].mptf.terraform_address}"
  module_source = try(one(data.modtm_module_source.telemetry).module_source, "")
  module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
} : {}%{ endif}
TAGS
```

If `try(local.all_resources[each.value].tags != "", false)` is true then there's already `tags` defined in the matched resource block, so we would wrap it with a `merge` function along with our generated tags. Otherwise, only our generated tags would be set to `tags`. 