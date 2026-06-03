# `reorder_attributes` Transform Block

The `reorder_attributes` transform block re-orders the attributes inside a single root block (`resource`, `data`, `variable`, `output`, `module`, `moved` or `terraform`). It only changes attribute order — it never adds, removes, or mutates attribute values.

This transform composes with `data` blocks like `data "resource"`, `data "variable"` and `data "output"` to express AVM-style "attributes must appear in this order" rules declaratively.

## Arguments

- `target_block_address`: The address of the block whose attributes you want to re-order, for example `resource.azurerm_storage_account.this` or `variable.location`.
- `head_attributes` *(optional)*: Names of attributes that should appear first, in the listed order.
- `tail_attributes` *(optional)*: Names of attributes that should appear last, in the listed order.

Either or both lists may be supplied. Attributes named in `head_attributes` are emitted first, attributes named in `tail_attributes` are emitted last, and everything else is written between them, preserving its original source-order position (or alphabetical for attributes added by earlier transforms that have no source position). Names that aren't present on the block are silently skipped. The same attribute appearing in both `head_attributes` and `tail_attributes` is a configuration error.

Nested blocks are preserved and re-emitted after the attributes.

## Example - Put `type` and `description` first on every variable

```terraform
data "variable" "all" {}

transform "reorder_attributes" "variables" {
  for_each             = data.variable.all.result
  target_block_address = "variable.${each.key}"
  head_attributes      = ["type", "description"]
}
```

Given:

```terraform
variable "location" {
  default     = "westeurope"
  description = "Azure region for the deployment"
  type        = string
}
```

After applying the transform:

```terraform
variable "location" {
  type        = string
  description = "Azure region for the deployment"
  default     = "westeurope"
}
```

## Example - Head and tail together

```terraform
transform "reorder_attributes" "resource" {
  target_block_address = "resource.azurerm_storage_account.this"
  head_attributes      = ["name", "resource_group_name", "location"]
  tail_attributes      = ["tags"]
}
```

`name`, `resource_group_name`, `location` are emitted first (in that order); `tags` is emitted last; every other attribute keeps its original relative position between them.

## Detailed Behavior

- The transform runs against the parsed HCL writer view of the block, so comments and formatting on individual attributes are preserved.
- If the existing attribute order already matches the computed order, the transform is a no-op and the file is left untouched.
- Nested blocks (for example a `lifecycle` block inside a `resource`) are not re-ordered — they are simply re-emitted after the attributes. Use a separate `move_block` or `append_block_body` transform if you need to manipulate nested-block layout.
