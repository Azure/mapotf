# `reorder_attributes` Transform Block

The `reorder_attributes` transform block re-orders the attributes **and nested blocks** inside a single root block (`resource`, `data`, `ephemeral`, `variable`, `output`, `module`, `moved` or `terraform`). It only changes layout — it never adds, removes, or mutates attribute values.

This transform composes with `data` blocks like `data "resource"`, `data "variable"`, `data "output"`, `data "provider_schema"` and `data "module_source"` to express AVM-style "attributes must appear in this order" rules declaratively.

> **Breaking renames in v0.1.4** — the section names are now `head` / `body` / `foot` (mirroring HTML `thead`/`tbody`/`tfoot`). Migrate every existing transform as follows:
>
> | v0.1.3 name | v0.1.4 name |
> |---|---|
> | `tail_attributes` | `foot_attributes` |
> | `sort_middle_alphabetically` | `sort_body_alphabetically` |
> | `head_tail_line_breaks` | `head_foot_line_breaks` |
>
> There are no aliases — the old names are rejected by the HCL decoder. Pin `mapotf` to `v0.1.4` and the new config in the same commit.

## Arguments

- `target_block_address`: The address of the block whose layout you want to re-order, for example `resource.azurerm_storage_account.this` or `variable.location`.
- `head_attributes` *(optional)*: Names of elements that should appear first, in the listed order.
- `body_attributes` *(optional)*: Names of elements that should appear in the **body** section (between head and foot) in the listed order. Names not in `head_attributes` / `body_attributes` / `foot_attributes` are still emitted after the listed body names — see the interaction table below for how `sort_body_alphabetically` controls their order.
- `foot_attributes` *(optional)*: Names of elements that should appear last, in the listed order.
- `head_foot_line_breaks` *(optional, default `true`)*: When `true`, a blank line is inserted between the head section and the body, and between the body and the foot section. Set to `false` to suppress those blank lines.
- `sort_body_alphabetically` *(optional, default `true`)*: Controls the order of body-section elements that are **not** listed in `body_attributes`. When `true`, those elements are sorted alphabetically by name. Set to `false` to preserve the original source order instead.
- `nested_block_path` *(optional)*: A non-empty list of nested block names below `target_block_address`. When set, the transform resolves exactly one block at every path segment and alphabetizes only the direct attributes in the selected block. This focused mode cannot be combined with the section-ordering arguments above, and the selected block cannot itself contain nested blocks.

### How `body_attributes` and `sort_body_alphabetically` interact

| `body_attributes` | `sort_body_alphabetically` | Behaviour for the body section |
|---|---|---|
| unset | `true` (default) | Entire body sorted alphabetically. |
| unset | `false` | Entire body in original source order. |
| `[a, b, c]` | `true` | `a`, `b`, `c` first (in that order, filtered to elements actually on the block); then every other body element sorted alphabetically. |
| `[a, b, c]` | `false` | `a`, `b`, `c` first (in that order); then every other body element in original source order. |

This makes schema-driven ordering safe: an attribute the schema doesn't yet know about is never lost — it just lands after the schema-derived names, in a predictable position.

### Nested blocks are elements too

Nested blocks (for example `lifecycle`, `network_interface`, or `dynamic "subnet"`) are treated exactly like attributes — list them in `head_attributes` / `body_attributes` / `foot_attributes` by their block type, or, for `dynamic` blocks, by their label (e.g. `subnet` for `dynamic "subnet" {}`).

A nested block in the output has a blank line in front of it when it is preceded by an attribute or a different kind of nested block, so they read naturally in Terraform style. Adjacent nested blocks of the same type (and, for `dynamic`, the same label) — for example two consecutive `validation { }` blocks under a `variable` — are kept adjacent with no blank line between them.

### Edge cases

- Names listed in `head_attributes` / `body_attributes` / `foot_attributes` that don't exist on the block are silently skipped.
- The same name appearing in more than one of `head_attributes`, `body_attributes`, or `foot_attributes` is a configuration error.
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

The blank line between the head (`type`, `description`) and the body (`default`) comes from the default `head_foot_line_breaks = true`.

## Example - Sort `required_providers` entries

Use `nested_block_path` to order provider entries without rewriting their values:

```terraform
transform "reorder_attributes" "required_providers" {
  target_block_address = "terraform"
  nested_block_path    = ["required_providers"]
}
```

Given:

```terraform
terraform {
  required_providers {
    random = {
      source = "hashicorp/random"
    }

    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}
```

After applying the transform, `azurerm` precedes `random`. The `source` and `version` keys inside each provider object are left exactly as written, along with the provider expression and its comments.

If a path segment is missing or has more than one matching nested block, the transform fails rather than choosing one. Multi-level paths are supported, for example `nested_block_path = ["first", "second"]`.

## Example - Head and foot together

```terraform
transform "reorder_attributes" "resource" {
  target_block_address = "resource.azurerm_storage_account.this"
  head_attributes      = ["name", "resource_group_name", "location"]
  foot_attributes      = ["tags"]
}
```

`name`, `resource_group_name`, `location` are emitted first (in that order); `tags` is emitted last; every other attribute is sorted alphabetically between them, separated from the head and foot by blank lines.

## Example - Nested block addressed in `head_attributes`

```terraform
transform "reorder_attributes" "vm" {
  target_block_address = "resource.azurerm_virtual_machine.this"
  head_attributes      = ["count", "for_each", "network_interface"]
  foot_attributes      = ["lifecycle", "depends_on"]
}
```

`network_interface` is a nested block; listing its type in `head_attributes` pulls it to the top alongside `count` / `for_each`. `lifecycle` (also a nested block) goes to the foot.

## Example - Disable section blanks

```terraform
transform "reorder_attributes" "module" {
  target_block_address  = "module.example"
  head_attributes       = ["source", "version"]
  foot_attributes       = ["depends_on"]
  head_foot_line_breaks = false
}
```

The head, body, and foot are emitted contiguously with no blank-line separators (nested blocks, if any, still get their own leading blank line).

## Example - Preserve source order for the body

```terraform
transform "reorder_attributes" "preserve_order" {
  target_block_address     = "resource.azurerm_storage_account.this"
  head_attributes          = ["name", "resource_group_name", "location"]
  foot_attributes          = ["tags"]
  sort_body_alphabetically = false
}
```

Attributes between the head and the foot keep the order they appeared in originally instead of being sorted alphabetically.

## Example - Schema-driven body ordering with `body_attributes`

```terraform
data "provider_schema" "azurerm" {
  provider_source  = "hashicorp/azurerm"
  provider_version = "~> 4.0"
}

transform "reorder_attributes" "resource_group_body" {
  target_block_address = "resource.azurerm_resource_group.this"
  head_attributes      = ["for_each", "count", "provider"]
  body_attributes = concat(
    try(data.provider_schema.azurerm.resources_required_attributes["azurerm_resource_group"], []),
    try(data.provider_schema.azurerm.resources_optional_attributes["azurerm_resource_group"], []),
  )
  foot_attributes = ["lifecycle", "depends_on"]
}
```

The body section emits provider-required attributes first (alphabetical within that group), then provider-optional attributes (alphabetical within that group). Any attribute on the block that isn't in either schema list still appears in the body — after the schema-listed names, sorted alphabetically by default — so a new provider release that adds an attribute can never silently drop it from the output.

## Detailed Behavior

- The transform runs against the parsed HCL writer view of the block, so comments and formatting on individual attributes are preserved.
- Nested blocks have a blank line in front of them when they are preceded by an attribute or a nested block of a different type; adjacent nested blocks of the same type (and, for `dynamic`, the same label) stay adjacent without a separating blank line. A section-boundary blank line counts — no extra blank line is added when one is already being emitted.
- Nested blocks of the same type that appear multiple times (e.g. two `network_interface` blocks) keep their write-side order when grouped together by name.
