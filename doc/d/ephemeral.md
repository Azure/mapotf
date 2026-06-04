# Data "ephemeral" Block

The `data "ephemeral"` block is used to query and retrieve `ephemeral` blocks from a Terraform configuration. Ephemeral resources were introduced in Terraform 1.10 for values that should not be persisted in state (e.g. tokens, short-lived credentials). This data block allows you to filter and collect ephemeral blocks based on specific criteria such as the ephemeral resource type, and whether `count` or `for_each` is used.

The shape and semantics mirror [`data "resource"`](resource.md) and [`data "data"`](data.md); only the queried block type differs.

## Arguments

- `ephemeral_type`: This optional argument specifies the type of the ephemeral resource to filter. It is a string attribute.
- `use_count`: This optional argument is a boolean that, when set to `true`, filters ephemeral blocks that use the `count` attribute. The default value is `false`.
- `use_for_each`: This optional argument is a boolean that, when set to `true`, filters ephemeral blocks that use the `for_each` attribute. The default value is `false`.

## Attributes

- `result`: This attribute contains the filtered ephemeral blocks, all expressions assigned to arguments are evaluated as strings.

## Example - Querying Ephemeral Blocks

Here is an example of how to use the `data "ephemeral"` block to query ephemeral resources of a specific type:

```terraform
data "ephemeral" "secrets" {
  ephemeral_type = "azurerm_key_vault_secret"
}
```

In this example, the `data "ephemeral"` block queries all ephemeral blocks of type `azurerm_key_vault_secret` and stores the result in the `secrets` data source.

## Example - Filtering Ephemeral Blocks with `count`

```terraform
data "ephemeral" "example" {
  ephemeral_type = "fake_ephemeral"
  use_count      = true
}
```

## Example - Filtering Ephemeral Blocks with `for_each`

```terraform
data "ephemeral" "example" {
  ephemeral_type = "fake_ephemeral"
  use_for_each   = true
}
```

## Example - Retrieving Results

Assuming we have such Terraform config:

```terraform
ephemeral "azurerm_key_vault_secret" "example" {
  name         = "db-password"
  key_vault_id = data.azurerm_key_vault.example.id
}
```

And our Mapotf config is:

```terraform
data "ephemeral" "example" {
  ephemeral_type = "azurerm_key_vault_secret"
}
```

The data source's results are aggregated by ephemeral type first, then by the block labels, identically to `data "data"` and `data "resource"`. Each block exposes its arguments as string-valued attributes plus the standard `mptf` metadata object (block address, labels, range, module reference).

Ephemeral root blocks are also addressable via `RootBlock("ephemeral.<type>.<name>")` and are therefore valid targets for transforms such as `move_block`, `reorder_attributes`, `sort_blocks_in_file`, etc.
