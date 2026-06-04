# `rename_block_element` Transform Block

The `rename_block_element` transform renames an attribute or a nested block on every `resource` or `data` block of a given type. It is the right tool for refactoring across a provider upgrade where one argument has been renamed but is otherwise compatible.

## Arguments

- `rename` *(nested block, one or more)*: Each `rename` block describes a single renaming operation. The transform applies them in declaration order.

### `rename` block

- `resource_type`: The Terraform resource or data type to operate on. Use the bare type for resources (for example `azurerm_storage_account`) and the `data.`-prefixed form for data sources (for example `data.azurerm_client_config`).
- `element_path`: The path to the attribute or nested block to rename, as a list of strings. The last element is the name to rename; earlier elements walk into nested blocks. For example `["nested_block", "attr"]` renames the `attr` attribute inside the `nested_block` nested block. To rename a nested block itself, use a single-element path equal to the nested block's type name (for example `["network_interface"]`).
- `new_name`: The new name for the attribute or nested block.
- `rename_only_new_name_absent` *(optional, default `false`)*: If `true`, the rename only happens when the destination name is not already set on the block. This is useful when both old and new names coexist on some resources and you want to preserve any explicit new-name value.
- `attribute_path` *(deprecated)*: Older spelling of `element_path`. Set exactly one of the two; both cannot be used together.

## Attributes

This transform has no readable attributes.

## Example - Rename a top-level attribute

```terraform
transform "rename_block_element" "rename_legacy_name" {
  rename {
    resource_type = "azurerm_storage_account"
    element_path  = ["legacy_name"]
    new_name      = "name"
  }
}
```

Given:

```terraform
resource "azurerm_storage_account" "this" {
  legacy_name = "stthis"
  location    = "eastus"
}
```

After applying the transform:

```terraform
resource "azurerm_storage_account" "this" {
  name     = "stthis"
  location = "eastus"
}
```

## Example - Rename a nested block

```terraform
transform "rename_block_element" "rename_blob_props" {
  rename {
    resource_type = "azurerm_storage_account"
    element_path  = ["blob_properties"]
    new_name      = "blob_properties_v2"
  }
}
```

This also rewrites `dynamic "blob_properties" {}` blocks to `dynamic "blob_properties_v2" {}`, preserving the original name as the `iterator` so existing `each.key` / `each.value` references continue to resolve.

## Example - Rename an attribute inside a nested block

```terraform
transform "rename_block_element" "rename_nested_attr" {
  rename {
    resource_type = "azurerm_kubernetes_cluster"
    element_path  = ["default_node_pool", "vm_size"]
    new_name      = "node_vm_size"
  }
}
```

Renames `vm_size` to `node_vm_size` inside every `default_node_pool` nested block of every `azurerm_kubernetes_cluster` resource.

## Example - Rename on a data source

```terraform
transform "rename_block_element" "data_source_rename" {
  rename {
    resource_type = "data.azurerm_resources"
    element_path  = ["resource_group"]
    new_name      = "resource_group_name"
  }
}
```

The `data.` prefix tells the transform to look in the data block set rather than the resource block set.

## Example - Multiple renames in one transform

```terraform
transform "rename_block_element" "provider_v4_renames" {
  rename {
    resource_type = "azurerm_storage_account"
    element_path  = ["enable_https_traffic_only"]
    new_name      = "https_traffic_only_enabled"
  }
  rename {
    resource_type = "azurerm_storage_account"
    element_path  = ["min_tls_version"]
    new_name      = "minimum_tls_version"
  }
}
```

Each `rename` block runs independently, so a single transform can batch every renamed argument from one provider upgrade.

## Detailed Behavior

- The transform iterates every block of `resource_type` in the target module and applies the rename if the source name is present.
- Renaming a nested block preserves its body unchanged. Renaming an attribute preserves its expression tokens, so complex expressions (interpolations, function calls) survive the rename verbatim.
- `dynamic` nested blocks are handled specially: the dynamic block's label is rewritten to `new_name` and the original name is set as the `iterator` to keep `<original_name>.key` / `<original_name>.value` references valid.
- When `rename_only_new_name_absent` is `true`, the old attribute is still removed only if the destination is set; this preserves blocks that already use the new name and quietly cleans up the obsolete one.
