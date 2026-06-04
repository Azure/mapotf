# Data "module" Block

The `data "module"` block enumerates `module` blocks in the target Terraform configuration. Each result entry is the module block's full evaluation context — its attribute values (stringified) plus an `mptf` metadata sub-object that includes the block's source file and range.

## Arguments

- `name` *(optional)*: If supplied, narrows the result to the single `module` block with this label. If omitted, every `module` block is returned.

## Attributes

- `result`: A map keyed by module label. Each value is the matching block's evaluation context (attributes plus `mptf` metadata).

## Example - Enumerate every module block

```terraform
data "module" "all" {}

locals {
  module_addresses = [for name, _ in data.module.all.result : "module.${name}"]
}
```

Given:

```terraform
module "naming" {
  source  = "Azure/naming/azurerm"
  version = "~> 0.4"
}

module "storage" {
  source = "./modules/storage"
}
```

`data.module.all.result` will contain two entries keyed `"naming"` and `"storage"`, and `local.module_addresses` will be `["module.naming", "module.storage"]`.

## Example - Look up a specific module

```terraform
data "module" "naming" {
  name = "naming"
}
```

`data.module.naming.result` will contain a single entry keyed `"naming"`, suitable for use in a `for_each` filter or a targeted transform such as `move_block` or `reorder_attributes`.

## Common Composition - Move every module block into main.tf

```terraform
data "module" "all" {}

transform "move_block" "modules_to_main" {
  for_each             = data.module.all.result
  target_block_address = "module.${each.key}"
  file_name            = "main.tf"
}
```

This pattern is used in the AVM pre-commit pipeline to enforce "module blocks live in main.tf, not in variables.tf or outputs.tf".
