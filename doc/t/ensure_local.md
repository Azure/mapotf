# `ensure_local` Transform Block

The `ensure_local` transform guarantees that a named local value exists and has a specific expression. If a matching `local.<name>` already exists in any `locals` block it is updated in place; if not, a new `locals` block is created in `fallback_file_name`.

This is the canonical primitive for "make sure this local is defined and equals this expression" rules — typical uses include injecting telemetry GUIDs, AVM headers, or any meta-programming value that downstream transforms depend on.

## Arguments

- `name`: The name of the local value to ensure. Must not include the `local.` prefix.
- `fallback_file_name`: The file to create the new `locals` block in if no `locals` block currently defines `name`. Should end in `.tf`. If a `locals` block defining `name` already exists, this argument is ignored.
- `value_as_string`: The new value, supplied as a string that will be parsed as an HCL expression. Use this when the expression is easier to write inside a string (for example, when it contains characters that would otherwise need escaping at the HCL level). Mutually exclusive with `value_as_raw`.
- `value_as_raw`: The new value, supplied as a raw HCL expression. Use this when the expression can be written inline — references to other variables, locals, function calls, and so on. Mutually exclusive with `value_as_string`.

Exactly one of `value_as_string` or `value_as_raw` must be set.

## Attributes

This transform has no readable attributes.

## Example - Ensure a local is present, creating a new locals block

```terraform
transform "ensure_local" "telemetry_id" {
  name               = "telemetry_id"
  fallback_file_name = "main.tf"
  value_as_string    = "\"00000000-0000-0000-0000-000000000000\""
}
```

Given a module with no `locals` block defining `telemetry_id`, the transform appends a new `locals` block to `main.tf`:

```terraform
locals {
  telemetry_id = "00000000-0000-0000-0000-000000000000"
}
```

## Example - Update an existing local in place

```terraform
transform "ensure_local" "module_source" {
  name               = "module_source"
  fallback_file_name = "main.tf"
  value_as_raw       = var.module_source
}
```

Given:

```terraform
# locals.tf
locals {
  module_source = "deprecated/source"
  other         = "unchanged"
}
```

After applying the transform, `locals.tf` still contains both locals; only `module_source` is rewritten and `fallback_file_name` is ignored because a definition already existed:

```terraform
# locals.tf
locals {
  module_source = var.module_source
  other         = "unchanged"
}
```

## Example - Use `value_as_string` to inject a multi-line expression

```terraform
transform "ensure_local" "tags" {
  name               = "tags"
  fallback_file_name = "main.tf"
  value_as_string    = "merge(var.tags, { managed_by = \"terraform\" })"
}
```

The string is parsed as an HCL expression at apply time, so the rendered file contains the expression unquoted:

```terraform
locals {
  tags = merge(var.tags, { managed_by = "terraform" })
}
```

## Detailed Behavior

- `ensure_local` looks up `local.<name>` across every `locals` block in the target module. The first match wins.
- When updating in place, only the `name` attribute on the matching `locals` block is replaced. Other attributes in the same block are untouched and their original ordering is preserved.
- When inserting a new `locals` block, the block is appended to `fallback_file_name`. The transform never inserts into an existing `locals` block — it only ever creates a new one, leaving the existing arrangement of files alone.
- `value_as_raw` accepts the raw token stream of the expression as it appears in the `.mptf.hcl` file. References like `var.foo`, `local.bar`, function calls, conditionals, and so on are all valid.
- `value_as_string` is parsed once at apply time; if the string is not a valid HCL expression the transform returns an error.
