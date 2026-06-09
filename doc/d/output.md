# Data "output" Block

The `data "output"` block enumerates `output` blocks in the target Terraform configuration. Each result entry is the output block's full evaluation context — its attribute values plus an `mptf` metadata sub-object that includes the block's source file and range.

Literal attribute values (string, number, bool, list, object) are decoded to their typed `cty` form, so HCL like `sensitive = false` exposes `each.value.sensitive` as the bool `false`. Attribute values that reference variables, locals, or resource attributes cannot be evaluated at config-load time and fall back to the literal token text — for example `value = azurerm_resource_group.this.id` exposes `each.value.value` as the string `"azurerm_resource_group.this.id"`.

## Arguments

- `name` *(optional)*: If supplied, narrows the result to the single `output` block with this label. If omitted, every `output` block is returned.

## Attributes

- `result`: A map keyed by output name. Each value is the matching block's evaluation context (attributes plus `mptf` metadata). Attributes the block does not declare (for example `sensitive` on a non-sensitive output) are simply absent from the map.

## Example - Enumerate every output block

```terraform
data "output" "all" {}

locals {
  output_addresses = sort([for name, _ in data.output.all.result : "output.${name}"])
}
```

Given:

```terraform
output "id" {
  value = azurerm_resource_group.this.id
}

output "name" {
  value = azurerm_resource_group.this.name
}
```

`data.output.all.result` contains two entries keyed `"id"` and `"name"`, and `local.output_addresses` is `["output.id", "output.name"]`.

## Example - Drop redundant `sensitive = false` declarations

`sensitive = false` is the Terraform default for outputs, so AVM treats it as redundant. Combine `data "output"` with `remove_block_element` to drop it everywhere it appears:

```terraform
data "output" "all" {}

transform "remove_block_element" "drop_output_sensitive_false" {
  for_each             = { for n, v in data.output.all.result : n => v if try(v.sensitive, true) == false }
  target_block_address = "output.${each.key}"
  paths                = ["sensitive"]
}
```

## Example - Sort every output alphabetically into outputs.tf

```terraform
data "output" "all" {}

locals {
  output_addresses = sort([for name, _ in data.output.all.result : "output.${name}"])
}

transform "sort_blocks_in_file" "outputs_tf" {
  file_name     = "outputs.tf"
  desired_order = local.output_addresses
}
```

This is the canonical AVM pre-commit rule for outputs — every `output` block ends up in `outputs.tf`, in alphabetical order, regardless of which file it was originally declared in.

## Detailed Behavior

- The result is a map keyed by output name, so for any given module each output appears at most once.
- The `mptf.range.file_name` field of each entry is useful for filtering blocks by source file (for example "every output currently in `main.tf` should move to `outputs.tf`").
- All attribute values are decoded to their typed `cty` form whenever the expression is a literal (string, number, bool, list, object). Bool attributes like `sensitive = false` surface as the bool `false`. Expressions that reference variables, locals, resource attributes, or functions cannot be evaluated at config-load time and fall back to the literal token text — `value = azurerm_resource_group.this.id`, for example, surfaces as the string `"azurerm_resource_group.this.id"`.
