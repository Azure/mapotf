# `sort_blocks_in_file` Transform Block

The `sort_blocks_in_file` transform block makes sure a named set of root blocks ends up in one specific `.tf` file, in one specific order. It is the building block behind AVM-style rules such as "every `variable` block must live in `variables.tf`, sorted alphabetically" or "every `output` block must live in `outputs.tf`, sorted alphabetically".

This transform is normally driven by `data "variable"`, `data "output"`, `data "module"`, `data "moved"` or `data "resource"` — the data source enumerates the blocks, the user expresses the ordering rule, and the transform applies it.

## Arguments

- `file_name`: The target `.tf` file (must end in `.tf`). Blocks listed in `desired_order` will be moved into this file, in the listed order. The file is created if it doesn't already exist.
- `desired_order`: A non-empty list of block addresses, in the order they should appear in `file_name`. Each address must resolve to a known block in the target module (for example `variable.location`, `output.id`, `module.naming`, `moved.0`). An address that does not resolve is a hard error — silent skipping would mask drift, since `desired_order` is almost always computed from a data source.

## Attributes

This transform has no readable attributes.

## Example - Sort every variable alphabetically into variables.tf

```terraform
data "variable" "all" {}

locals {
  variable_addresses = sort([for name, _ in data.variable.all.result : "variable.${name}"])
}

transform "sort_blocks_in_file" "variables_tf" {
  file_name     = "variables.tf"
  desired_order = local.variable_addresses
}
```

Given two source files:

```terraform
# main.tf
variable "tags" {
  type = map(string)
}

# variables.tf
variable "location" {
  type = string
}
variable "name" {
  type = string
}
```

After applying the transform, `variables.tf` contains the three `variable` blocks in alphabetical order and the `variable "tags"` block has been removed from `main.tf`:

```terraform
# variables.tf
variable "location" {
  type = string
}
variable "name" {
  type = string
}
variable "tags" {
  type = map(string)
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

## Detailed Behavior

- The transform is a two-pass operation: every block listed in `desired_order` is first removed from whichever file currently holds it, then re-added to `file_name` in the listed order. This makes it safe to use even when the source and target files overlap.
- Blocks that already live in `file_name` but are *not* listed in `desired_order` are left untouched at the top of the file. This matches the behaviour expected by AVM pre-commit, where the sort rule only owns one block type at a time.
- The list ordering is exactly the order of `desired_order` — the transform does not sort the list for you. Use HCL's `sort()` function (as in the examples above) if you want alphabetical order.
