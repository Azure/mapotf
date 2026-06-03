# `move_block` Transform Block

The `move_block` transform moves one root block to a specified `.tf` file. It is the per-block primitive that `sort_blocks_in_file` uses internally — reach for `move_block` when you want to relocate an individual block without enforcing a complete file ordering.

## Arguments

- `target_block_address`: The address of the root block to move (for example `variable.location`, `output.id`, `resource.azurerm_resource_group.this`, `module.naming`). If the address does not resolve to a known block the transform returns an error.
- `file_name`: The destination `.tf` file. Must end in `.tf`. The file is created if it doesn't already exist.

## Attributes

This transform has no readable attributes.

## Example - Move a single variable into variables.tf

```terraform
transform "move_block" "location_to_variables" {
  target_block_address = "variable.location"
  file_name            = "variables.tf"
}
```

Given:

```terraform
# main.tf
variable "location" {
  type = string
}

resource "azurerm_resource_group" "this" {
  name     = "rg-this"
  location = var.location
}
```

After applying the transform:

```terraform
# main.tf
resource "azurerm_resource_group" "this" {
  name     = "rg-this"
  location = var.location
}

# variables.tf
variable "location" {
  type = string
}
```

## Example - Consolidate every variable into variables.tf via `for_each`

```terraform
data "variable" "all" {}

transform "move_block" "variables_to_variables_tf" {
  for_each             = data.variable.all.result
  target_block_address = "variable.${each.key}"
  file_name            = "variables.tf"
}
```

This is the same idea as `sort_blocks_in_file` without the ordering — every `variable` block ends up in `variables.tf`, preserving each block's original relative position among other variables.

## Example - Move misplaced blocks out of variables.tf

```terraform
data "resource" "all" {}

locals {
  resources_in_variables_tf = flatten([
    for type, rs in data.resource.all.result : [
      for name, b in rs : "resource.${type}.${name}"
      if b.mptf.range.file_name == "variables.tf"
    ]
  ])
}

transform "move_block" "resources_out_of_variables_tf" {
  for_each             = { for addr in local.resources_in_variables_tf : addr => addr }
  target_block_address = each.value
  file_name            = "main.tf"
}
```

This relocates every `resource` block that has wandered into `variables.tf` back into `main.tf`. The same pattern works for `output`, `module`, and `data` blocks — combine multiple `move_block` transforms to fully replicate the AVM "stray blocks belong somewhere else" rule.

## Detailed Behavior

- If the target block is already in `file_name`, the transform is a no-op.
- Otherwise the block is appended to `file_name` (creating the file if needed) and removed from its original file. The transform does not preserve a specific ordering within the destination file — use `sort_blocks_in_file` when ordering matters.
- The transform is per-block. If you need to move many blocks deterministically into a single file in a specific order, `sort_blocks_in_file` is a single declarative call that does the same work without `for_each`.
