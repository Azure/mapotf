# Data "variable" Block

The `data "variable"` block enumerates `variable` blocks in the target Terraform configuration. Each result entry is the variable block's full evaluation context — its attribute values (stringified) plus an `mptf` metadata sub-object that includes the block's source file and range.

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
  for_each             = { for n, v in data.variable.all.result : n => v if try(v.nullable, "") == "true" }
  target_block_address = "variable.${each.key}"
  paths                = ["nullable"]
}
```

The `try(v.nullable, "")` guard handles variables that don't declare `nullable` at all.

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
- All attribute values are returned as their string representation. Numeric defaults like `default = 5` show up as the string `"5"`; type expressions like `type = string` show up as the string `"string"`; complex defaults like `default = { a = 1 }` show up as the literal text of the expression.
