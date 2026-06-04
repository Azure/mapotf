# `remove_block` Transform Block

The `remove_block` transform removes an entire root block from the target Terraform configuration. It is the coarsest of the removal transforms — use `remove_block_element` if you want to remove only a nested block or attribute, and keep the surrounding block intact.

## Arguments

- `target_block_address`: The address of the root block to remove (for example `resource.azurerm_resource_group.this`, `variable.legacy`, `module.naming`). If the address does not resolve to a known block the transform returns an error.

## Attributes

This transform has no readable attributes.

## Example - Remove a single resource block

```terraform
transform "remove_block" "drop_legacy_rg" {
  target_block_address = "resource.azurerm_resource_group.legacy"
}
```

Given:

```terraform
resource "azurerm_resource_group" "this" {
  name     = "rg-this"
  location = "eastus"
}

resource "azurerm_resource_group" "legacy" {
  name     = "rg-legacy"
  location = "eastus"
}
```

After applying the transform:

```terraform
resource "azurerm_resource_group" "this" {
  name     = "rg-this"
  location = "eastus"
}
```

## Example - Remove every block of a type via `for_each`

```terraform
data "variable" "all" {}

locals {
  legacy_variables = {
    for name, v in data.variable.all.result : name => v
    if startswith(name, "legacy_")
  }
}

transform "remove_block" "drop_legacy_vars" {
  for_each             = local.legacy_variables
  target_block_address = "variable.${each.key}"
}
```

This removes every `variable` block whose name starts with `legacy_`, regardless of which file it currently lives in.

## Detailed Behavior

- The transform locates the block by its address (the same address format used everywhere else in mapotf — `resource.<type>.<name>`, `data.<type>.<name>`, `variable.<name>`, `output.<name>`, `local.<name>`, `module.<name>`, `moved.<index>`).
- Once located, the block is removed from whichever file currently holds it. If that leaves the file empty, the file is rewritten as empty — mapotf does not delete empty files automatically.
- The transform is destructive and there is no undo. Pair it with version control review the way you would any other refactor.
