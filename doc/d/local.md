# Data "local" Block

The `data "local"` block enumerates the attributes defined across every `locals` block in the target Terraform configuration. Unlike `data "variable"` or `data "output"`, the result is keyed by local *name* — `locals` blocks have no labels of their own, so all local values from all blocks are flattened into a single map.

## Arguments

- `name` *(optional)*: If supplied, only `locals` blocks that contain a local with this exact name are scanned, and only the matching attribute is returned. This is a coarse filter — when several `locals` blocks each define the same name (which Terraform itself forbids at apply time), all matches are returned, with later definitions overwriting earlier ones in the result map.

## Attributes

- `result`: A map keyed by local name. Each value is the local's expression rendered as a string.

## Example - Enumerate every local

```terraform
data "local" "all" {}

locals {
  local_names = sort(keys(data.local.all.result))
}
```

Given:

```terraform
# main.tf
locals {
  module_source = "Azure/avm-res-storage-storageaccount/azurerm"
  module_version = "0.2.4"
}

# tags.tf
locals {
  tags = {
    managed_by = "terraform"
  }
}
```

`data.local.all.result` contains three entries — `module_source`, `module_version`, and `tags` — and `local.local_names` is `["module_source", "module_version", "tags"]`.

## Example - Check whether a specific local is already defined

```terraform
data "local" "telemetry_id" {
  name = "telemetry_id"
}

transform "ensure_local" "inject_telemetry" {
  # Only ensure if not already defined elsewhere.
  count              = contains(keys(data.local.telemetry_id.result), "telemetry_id") ? 0 : 1
  name               = "telemetry_id"
  fallback_file_name = "main.tf"
  value_as_string    = "\"00000000-0000-0000-0000-000000000000\""
}
```

This pattern lets you guard an `ensure_local` transform on the absence of a pre-existing definition. (In most cases `ensure_local` is idempotent enough that the guard isn't necessary — it updates in place when a definition exists.)

## Detailed Behavior

- The result is a flat `map(string)`: keys are local names, values are the stringified HCL expression assigned to each local in source.
- The data source ignores the layout of `locals` blocks. Two blocks each defining `foo` and `bar` appear identically to one block defining both — only the names and expressions are exposed.
- Expression values are returned as their source text, not as evaluated values. `tags = merge(var.tags, { foo = "bar" })` shows up as the string `"merge(var.tags, { foo = \"bar\" })"`.
- mapotf does not enforce or guarantee a deterministic order across `locals` blocks when the same name is defined twice. Avoid relying on cross-block name collisions.
