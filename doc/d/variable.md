# Data "variable" Block

The `data "variable"` block enumerates `variable` blocks in the target Terraform configuration. Each result entry is the variable block's full evaluation context — its attribute values plus an `mptf` metadata sub-object that includes the block's source file and range.

Literal attribute values (string, number, bool, list, object) are decoded to their typed `cty` form, so HCL like `default = 5` exposes `each.value.default` as the number `5` and `nullable = true` as the bool `true`. Attribute values that reference variables, locals, functions, or `each.*` cannot be evaluated at config-load time and fall back to the literal token text — for example `default = var.something` exposes `each.value.default` as the string `"var.something"`. The `type` attribute is almost always a bare type expression (`string`, `map(string)`, ...) so it usually surfaces as its token text.

## Arguments

- `name` *(optional)*: If supplied, narrows the result to the single `variable` block with this label. If omitted, every `variable` block is returned.
- `type` *(optional)*: If supplied, filters the result to `variable` blocks whose `type` attribute matches this string exactly (for example `string`, `map(string)`, `list(object({ name = string }))`). The comparison is on the stringified token form.

## Attributes

- `result`: A map keyed by variable name. Each value is the matching block's evaluation context (attributes plus `mptf` metadata). Attributes the block does not declare (for example `default` on a required variable) are simply absent from the map — use `contains(keys(v), "default")` to detect them.

## Example - Enumerate every variable block

```terraform
data "variable" "all" {}

locals {
  variable_addresses = [for name, _ in data.variable.all.result : "variable.${name}"]
}
```

Given:

```terraform
variable "location" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}
```

`data.variable.all.result` contains two entries keyed `"location"` and `"tags"`, and `local.variable_addresses` is `["variable.location", "variable.tags"]`.

## Example - Detect required vs optional variables

A variable is "required" in Terraform terms when it has no `default` attribute. The data source exposes the full attribute set, so the distinction is purely an HCL filter:

```terraform
data "variable" "all" {}

locals {
  required_names = sort([for n, v in data.variable.all.result : n if !contains(keys(v), "default")])
  optional_names = sort([for n, v in data.variable.all.result : n if  contains(keys(v), "default")])
}
```

## Example - Drop redundant `nullable = true` declarations

`nullable = true` is the Terraform default for variables, so AVM treats it as redundant. Combine `data "variable"` with `remove_block_element` to drop it everywhere it appears:

```terraform
data "variable" "all" {}

transform "remove_block_element" "drop_nullable_true" {
  for_each             = { for n, v in data.variable.all.result : n => v if try(v.nullable, false) == true }
  target_block_address = "variable.${each.key}"
  paths                = ["nullable"]
}
```

The `try(v.nullable, false)` guard handles variables that don't declare `nullable` at all by defaulting to `false`, so the filter only fires on `nullable = true`.

## Example - Look up a single variable

```terraform
data "variable" "location" {
  name = "location"
}
```

`data.variable.location.result` contains a single entry keyed `"location"`, suitable for use in a targeted transform such as `move_block` or `update_in_place`.

## Detailed Behavior

- The result is a map keyed by variable name, so for any given module each variable appears at most once.
- The `mptf.range.file_name` field of each entry is useful for filtering blocks by source file (for example "every variable currently in `main.tf` should move to `variables.tf`").
- All attribute values are decoded to their typed `cty` form whenever the expression is a literal (string, number, bool, list, object, heredoc without interpolation). Numeric defaults like `default = 5` surface as the number `5`; bool defaults like `nullable = true` surface as the bool `true`; complex defaults like `default = { a = 1 }` surface as an object you can index (for example `each.value.default.a`). Expressions that reference variables, locals, functions, or iterators cannot be evaluated at config-load time and fall back to the literal token text — for example `default = var.something` surfaces as the string `"var.something"`. The `type` attribute is almost always a bare type expression (`type = string`) and surfaces as its token text (`"string"`).
