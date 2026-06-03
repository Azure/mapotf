# Plan: Native sort_blocks transform + goreleaser release

## Problem

`avm-terraform-governance/porch-configs/pre-commit.porch.yaml` runs two tools today:
1. `mapotf transform` — HCL meta-programming (telemetry, headers, provider versions)
2. `avmfix -folder .` — reorders blocks/attributes to AVM spec

The goal is to replace avmfix natively in mapotf as a new transform type, remove avmfix as a dependency, and add a multi-platform goreleaser release workflow.

---

## Questions answered

### Does `required_first` require provider schema?

No. For `variable` blocks, "required" means the block has no `default` attribute — this is
deterministic from the HCL AST alone. No provider schema, no `terraform init` needed.

  - Variable with no `default` → required input (user must provide it)
  - Variable with a `default`  → optional input (has a fallback)

With `required_first = true`: required variables sort first (alphabetically within that group),
then optional variables (alphabetically within their group). Purely HCL inspection.

### Provider schema for resource/data block attribute ordering?

avmfix calls `terraform providers schema -json` to determine canonical attribute order for
`resource` and `data` blocks. mapotf does not currently have this capability.

However, the current avmfix usage in `pkg/transform_new_block.go` already passes
`&hcl.File{}` (empty schema) — meaning schema-based ordering is already a no-op today.

Conclusion: resource/data schema-driven ordering is **out of scope** for this plan. The
`sort_blocks` transform supports a static `attribute_order` list for any block type.

### Why not conflate `move_to_file` with `sort_blocks`?

The existing `transform "move_block"` already moves individual blocks between files.
`sort_blocks` should be a single-responsibility transform: sort block order and
reorder attributes within blocks. File-movement concerns are separate.

---

## avmfix capabilities — full mapping

| avmfix behaviour | Supported in plan? | mapotf mechanism |
|---|---|---|
| Sort `variable` blocks: required first, then alphabetical | ✅ | `sort_blocks` with `required_first = true` |
| Order attributes within `variable` blocks | ✅ | `sort_blocks` with `attribute_order` |
| Remove `nullable = true` from variable blocks | ✅ | `sort_blocks` with `remove_attribute` |
| Remove `sensitive = false` from variable blocks | ✅ | `sort_blocks` with `remove_attribute` |
| Sort `output` blocks: alphabetical | ✅ | `sort_blocks` with `sort_by_label = true` |
| Order attributes within `output` blocks | ✅ | `sort_blocks` with `attribute_order` |
| Remove `sensitive = false` from output blocks | ✅ | `sort_blocks` with `remove_attribute` |
| Order static `module` block attributes (source, version, etc.) | ✅ | `sort_blocks` with `attribute_order` |
| Order `module` inputs: required vars before optional vars | ⏳ deferred | Requires reading source module's variable definitions |
| Order `moved` block attributes (`from`, `to`) | ✅ | `sort_blocks` with `attribute_order` |
| Resource/data attribute ordering by provider schema | ❌ out of scope | Requires provider schema lookup |
| Move non-variable blocks out of `variables.tf` | existing | `transform "move_block"` per-block |
| Move non-output blocks out of `outputs.tf` | existing | `transform "move_block"` per-block |
| Sort `local` blocks | 🚫 not wanted | Not implemented per user requirement |

---

## New transform type: `sort_blocks`

### Responsibility

Sort blocks by label and/or reorder attributes within blocks. No file movement.
Module-scoped (no `for_each`), acts on all matching blocks at once.

### HCL schema

```hcl
transform "sort_blocks" "<name>" {
  # Required: which Terraform block type to act on
  block_type = "variable"   # variable | output | resource | data | module | moved

  # Sort blocks alphabetically by their label
  sort_by_label = true

  # (variable only) Blocks without a "default" attribute sort before those with one
  required_first = true

  # Reorder attributes within each block; unlisted attrs are appended in original order
  attribute_order = ["type", "default", "description", "nullable", "sensitive"]

  # Remove attributes whose value matches (can repeat for multiple rules)
  remove_attribute {
    name  = "nullable"
    value = "true"
  }
  remove_attribute {
    name  = "sensitive"
    value = "false"
  }
}
```

---

## Usage examples

### 1. Sort variable blocks (AVM spec)

```hcl
transform "sort_blocks" "sort_variables" {
  block_type     = "variable"
  sort_by_label  = true
  required_first = true

  attribute_order = ["type", "default", "description", "nullable", "sensitive"]

  remove_attribute {
    name  = "nullable"
    value = "true"
  }
  remove_attribute {
    name  = "sensitive"
    value = "false"
  }
}
```

**Before:**
```hcl
# in main.tf (misplaced - use move_block separately)
variable "tags" {
  sensitive   = false
  nullable    = true
  type        = map(string)
  description = "Tags to apply."
  default     = {}
}

variable "name" {
  description = "The resource name."
  type        = string
}
```

**After** (blocks sorted, required first; attributes reordered; redundant defaults removed):
```hcl
variable "name" {
  type        = string
  description = "The resource name."
}

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Tags to apply."
}
```

### 2. Sort output blocks (AVM spec)

```hcl
transform "sort_blocks" "sort_outputs" {
  block_type    = "output"
  sort_by_label = true

  attribute_order = ["value", "description", "sensitive", "depends_on"]

  remove_attribute {
    name  = "sensitive"
    value = "false"
  }
}
```

**Before:**
```hcl
variable "resource_id" {
  sensitive   = false
  value       = azurerm_resource_group.this.id
  description = "The resource group ID."
}

variable "name" {
  value = azurerm_resource_group.this.name
}
```

**After:**
```hcl
output "name" {
  value = azurerm_resource_group.this.name
}

output "resource_id" {
  value       = azurerm_resource_group.this.id
  description = "The resource group ID."
}
```

### 3. Order module block attributes (AVM spec)

The known meta-attributes are ordered first; remaining attributes (module inputs) stay
in their original relative order. Required/optional input distinction is deferred.

```hcl
transform "sort_blocks" "order_module_attrs" {
  block_type = "module"

  # Meta-attrs first; module-specific inputs follow in original order
  attribute_order = ["for_each", "count", "source", "version", "providers", "depends_on"]
}
```

**Before:**
```hcl
module "storage" {
  account_name = var.storage_name
  source       = "./modules/storage"
  depends_on   = [azurerm_resource_group.this]
  version      = "1.0.0"
  for_each     = var.storage_accounts
}
```

**After:**
```hcl
module "storage" {
  for_each     = var.storage_accounts
  source       = "./modules/storage"
  version      = "1.0.0"
  depends_on   = [azurerm_resource_group.this]
  account_name = var.storage_name
}
```

### 4. Order `moved` block attributes

```hcl
transform "sort_blocks" "order_moved_attrs" {
  block_type      = "moved"
  attribute_order = ["from", "to"]
}
```

### 5. Order resource/data block attributes (static list, no schema)

Provider-schema ordering is out of scope, but users can specify a static list:

```hcl
transform "sort_blocks" "order_resource_attrs" {
  block_type      = "resource"
  attribute_order = ["name", "location", "resource_group_name", "tags"]
}
```

Attributes not in the list are appended in their original relative order.

---

## File changes

| File | Action |
|---|---|
| `pkg/transform_sort_blocks.go` | New — `SortBlocksTransform` struct + `Apply()` |
| `pkg/transform_sort_blocks_test.go` | New — unit tests |
| `pkg/transform_new_block.go` | Replace avmfix calls with native hclwrite attribute sorting |
| `go.mod` / `go.sum` | Remove `github.com/lonegunmanb/avmfix` |
| `.goreleaser.yaml` | New — multi-platform build (6 targets, no signing) |
| `.github/workflows/release.yml` | New — release workflow on `v*.*.*` tags |

---

## Todos

| id | title |
|---|---|
| sort-blocks-transform | Implement `transform "sort_blocks"` in `pkg/transform_sort_blocks.go` |
| sort-blocks-test | Add tests in `pkg/transform_sort_blocks_test.go` |
| replace-avmfix-inline | Replace avmfix calls in `transform_new_block.go` with native hclwrite sorting |
| remove-avmfix-dep | Run `go mod tidy` to remove avmfix from go.mod |
| goreleaser-config | Add `.goreleaser.yaml` |
| release-workflow | Add `.github/workflows/release.yml` |

Dependencies: `remove-avmfix-dep` depends on `sort-blocks-transform` + `replace-avmfix-inline`.

---

## goreleaser config

`.goreleaser.yaml`:
- Builds: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64
- Archives: `zip` for Windows, `tar.gz` for all others
- Checksums file included
- No code signing (unsigned)

`.github/workflows/release.yml`:
- Trigger: `push` to tags matching `v*.*.*`
- Steps: `actions/checkout` (fetch-depth: 0), `actions/setup-go` (go-version-file: go.mod), `goreleaser/goreleaser-action` (args: `release --clean`)
- Permissions: `contents: write`

---

## avm-terraform-governance changes (separate PR in that repo)

In `porch-configs/pre-commit.porch.yaml`:
- Remove `avmfix` command_group and three-part avmfix serial step
- Add `sort_variables.mptf.hcl` and `sort_outputs.mptf.hcl` under `mapotf-configs/pre-commit/`
- Optionally add `order_module_attrs.mptf.hcl` and `order_moved_attrs.mptf.hcl`
- These are picked up by the existing `mapotf transform` step — no new command needed
