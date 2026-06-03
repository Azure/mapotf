# `reorder_attributes` Transform Block

The `reorder_attributes` transform block re-orders the attributes **and nested blocks** inside a single root block (`resource`, `data`, `variable`, `output`, `module`, `moved` or `terraform`). It only changes layout — it never adds, removes, or mutates attribute values.

This transform composes with `data` blocks like `data "resource"`, `data "variable"` and `data "output"` to express AVM-style "attributes must appear in this order" rules declaratively.

## Arguments

- `target_block_address`: The address of the block whose layout you want to re-order, for example `resource.azurerm_storage_account.this` or `variable.location`.
- `head_attributes` *(optional)*: Names of elements that should appear first, in the listed order.
- `tail_attributes` *(optional)*: Names of elements that should appear last, in the listed order.
- `head_tail_line_breaks` *(optional, default `true`)*: When `true`, a blank line is inserted between the head section and the middle, and between the middle and the tail section. Set to `false` to suppress those blank lines.
- `sort_middle_alphabetically` *(optional, default `true`)*: When `true`, every element that is not in `head_attributes` or `tail_attributes` is sorted alphabetically by name. Set to `false` to preserve the original source order instead.

### Nested blocks are elements too

Nested blocks (for example `lifecycle`, `network_interface`, or `dynamic "subnet"`) are treated exactly like attributes — list them in `head_attributes` / `tail_attributes` by their block type, or, for `dynamic` blocks, by their label (e.g. `subnet` for `dynamic "subnet" {}`).

A nested block in the output always has a blank line in front of it, even when no head/tail section boundary applies, so they read naturally in Terraform style.

### Edge cases

- Names listed in `head_attributes` / `tail_attributes` that don't exist on the block are silently skipped.
- The same name appearing in both `head_attributes` and `tail_attributes` is a configuration error.
- For attributes added by an earlier transform (no source position), the source-order mode sorts them alphabetically among themselves, after every element that has a source position.

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

  default = "westeurope"
}
```

The blank line between the head (`type`, `description`) and the middle (`default`) comes from the default `head_tail_line_breaks = true`.

## Example - Head and tail together

```terraform
transform "reorder_attributes" "resource" {
  target_block_address = "resource.azurerm_storage_account.this"
  head_attributes      = ["name", "resource_group_name", "location"]
  tail_attributes      = ["tags"]
}
```

`name`, `resource_group_name`, `location` are emitted first (in that order); `tags` is emitted last; every other attribute is sorted alphabetically between them, separated from the head and tail by blank lines.

## Example - Nested block addressed in `head_attributes`

```terraform
transform "reorder_attributes" "vm" {
  target_block_address = "resource.azurerm_virtual_machine.this"
  head_attributes      = ["count", "for_each", "network_interface"]
  tail_attributes      = ["lifecycle", "depends_on"]
}
```

`network_interface` is a nested block; listing its type in `head_attributes` pulls it to the top alongside `count` / `for_each`. `lifecycle` (also a nested block) goes to the tail.

## Example - Disable section blanks

```terraform
transform "reorder_attributes" "module" {
  target_block_address  = "module.example"
  head_attributes       = ["source", "version"]
  tail_attributes       = ["depends_on"]
  head_tail_line_breaks = false
}
```

The head, middle, and tail are emitted contiguously with no blank-line separators (nested blocks, if any, still get their own leading blank line).

## Example - Preserve source order for the middle

```terraform
transform "reorder_attributes" "preserve_order" {
  target_block_address       = "resource.azurerm_storage_account.this"
  head_attributes            = ["name", "resource_group_name", "location"]
  tail_attributes            = ["tags"]
  sort_middle_alphabetically = false
}
```

Attributes between the head and the tail keep the order they appeared in originally instead of being sorted alphabetically.

## Detailed Behavior

- The transform runs against the parsed HCL writer view of the block, so comments and formatting on individual attributes are preserved.
- Nested blocks always have a blank line in front of them in the output (a section-boundary blank line counts — no extra blank line is added when one is already being emitted).
- Nested blocks of the same type that appear multiple times (e.g. two `network_interface` blocks) keep their write-side order when grouped together by name.

