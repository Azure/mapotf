# Data "moved" Block

The `data "moved"` block enumerates `moved` blocks in the target Terraform configuration. Because a `moved` block has no native HCL label, each result is keyed by a synthetic, declaration-order index — `"0"` for the first `moved` block (in sort-order of file name), `"1"` for the second, and so on. The same index is also the suffix in the block address used by other transforms, for example `moved.0`.

Each result value is the block's evaluation context — its `from` and `to` attribute values (stringified) plus an `mptf` metadata sub-object that includes the block's source file and range.

## Arguments

This data source takes no arguments.

## Attributes

- `result`: A map keyed by synthetic index (`"0"`, `"1"`, ...). Each value is the matching `moved` block's evaluation context.

## Example - Enumerate every moved block

```terraform
data "moved" "all" {}

locals {
  moved_addresses = sort([for idx, _ in data.moved.all.result : "moved.${idx}"])
}
```

Given:

```terraform
# moved.tf
moved {
  from = azurerm_resource_group.legacy
  to   = azurerm_resource_group.this
}

moved {
  from = azurerm_storage_account.legacy
  to   = azurerm_storage_account.this
}
```

`data.moved.all.result` will contain two entries keyed `"0"` and `"1"`, and `local.moved_addresses` will be `["moved.0", "moved.1"]`.

## Common Composition - Sort every moved block into moved.tf

```terraform
data "moved" "all" {}

locals {
  moved_addresses = sort([for idx, _ in data.moved.all.result : "moved.${idx}"])
}

transform "sort_blocks_in_file" "moved_tf" {
  file_name     = "moved.tf"
  desired_order = local.moved_addresses
}
```

This is the pattern used by AVM pre-commit to keep every `moved` block in one consistent file.

## Detailed Behavior

- The synthetic index is assigned in deterministic, platform-independent order: source files are sorted by name and `moved` blocks are then numbered in the order they appear within each file.
- Because the index is positional, adding or removing a `moved` block earlier in the source will shift the indices of every subsequent block. Treat the addresses as ephemeral — compute them from `data.moved.all.result` rather than hard-coding them in transforms.
