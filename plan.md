# Plan: Replace avmfix with native mapotf transforms + multi-platform release

## Problem

`avm-terraform-governance/porch-configs/pre-commit.porch.yaml` runs two tools today:
1. `mapotf transform` — HCL meta-programming (telemetry, headers, provider versions)
2. `avmfix -folder .` — re-orders blocks and attributes to AVM spec; moves blocks between files

Goals:
1. Re-implement every behaviour AVM relies on in `avmfix` as native mapotf transforms
2. Remove `github.com/lonegunmanb/avmfix` as a Go dependency
3. Release mapotf as an unsigned multi-platform binary via goreleaser

---

## How mapotf actually works (the design constraints)

This plan is written around the existing mapotf paradigm. Each new transform follows the
patterns already established by `move_block`, `update_in_place`, `remove_block_element`, etc.

### Mechanism 1 — per-block transforms with `target_block_address`

Every transform that mutates a single addressable Terraform block accepts a
`target_block_address` string (e.g. `"variable.location"`, `"output.id"`,
`"resource.azurerm_resource_group.this"`, `"module.naming"`).

Example (existing `move_block`):
```hcl
transform "move_block" "x" {
  target_block_address = "variable.location"
  file_name            = "variables.tf"
}
```

### Mechanism 2 — `for_each` to iterate per-block transforms across many blocks

mapotf inherits golden's `for_each` meta-argument. The canonical example is
`example/tracing_tags/main.mptf.hcl` — one `update_in_place` transform is fanned out across
every resource block:

```hcl
data "resource" "all" {}

locals {
  addresses = [for t, rs in data.resource.all.result : [for n, _ in rs : "resource.${t}.${n}"]]
  flat_addresses = flatten(local.addresses)
}

transform "update_in_place" "tags" {
  for_each             = try(local.flat_addresses, [])
  target_block_address = each.value
  asstring {
    tags = "merge(var.tags, ...)"
  }
}
```

### Mechanism 3 — `data` blocks expose the target module's HCL

mapotf already exposes:
- `data "variable" all` → `result` is `map[var_name] => object` where the object contains
  every attribute of the variable block as a string (plus an `mptf` metadata sub-object)
- `data "output" all` → same shape for outputs
- `data "resource" all` → nested: `result.<type>.<name>` → object
- `data "data" all` → nested: `result.<type>.<name>` → object
- `data "local"` → all `locals` attributes
- `data "terraform"` → the `terraform { ... }` block (singular)
- `data "provider_schema"` (with `provider_source` + `provider_version`) → calls
  `terraform providers schema -json` and exposes `resources.<type>.block.attributes.<name>`
  with `required` / `optional` / `computed` / `sensitive` properties

This means:
- "Required vs optional variable" (no `default` attribute) is detectable in HCL alone:
  `!contains(keys(v), "default")`
- Schema-driven sort of resource attributes IS achievable in HCL using `data.provider_schema`
  (the current `transform_new_block.go` already imports avmfix but passes `&hcl.File{}` —
  the schema-driven path is already a no-op today)

The two data sources we are missing are `data "module"` and `data "moved"` — both small
additions modelled after `data "resource"`.

### Mechanism 4 — file-level state lives on `pkg/terraform.Module.writeFiles`

`writeFiles map[string]*hclwrite.File` is the in-memory model of every `.tf` file in the
target module. `cfg.AddBlock(filename, block)` appends; `cfg.module.RemoveBlock(block)`
removes the first match by type + labels. There is no built-in primitive for re-ordering
blocks within a file — that is the one new file-level operation this plan introduces.

### Separation-of-concerns rules followed by this plan

| Concern | Existing or new transform | One job |
|---|---|---|
| Move one block to a file | `move_block` (existing) | Single block + single file |
| Remove an attribute / nested block from one block | `remove_block_element` (existing) | Single block + paths |
| Conditional version of the above | the same transform, gated by a `for_each` map | Composition, not a new transform |
| Re-order attributes inside one block | **`reorder_attributes` (new)** | Single block + ordering |
| Re-order blocks inside one file | **`sort_blocks_in_file` (new)** | Single file + ordering |
| Expose `module` / `moved` blocks to HCL | **`data "module"`, `data "moved"` (new)** | Read-only data |
| Replace avmfix-formatted output in `new_block` | Rewrite `Format()` in `transform_new_block.go` | Internal cleanup |

Attribute ordering and file movement are **never combined** in the same transform.

---

## Complete avmfix capability mapping

| # | avmfix capability | mapotf replacement | New code |
|---|---|---|---|
| 1 | `VariablesFile.AutoFix`: sort variable blocks (required first, then alphabetical) inside `variables.tf` | `sort_blocks_in_file` with `desired_order` computed in HCL from `data.variable.all.result` | `sort_blocks_in_file` |
| 2 | `HclFile.AutoFix`: move stray `variable` blocks into `variables.tf` | `move_block` + `for_each = data.variable.all.result` | none (existing) |
| 3 | `VariablesFile.AutoFix`: move non-variable blocks out of `variables.tf` into `main.tf` | `move_block` + `for_each` over each block kind that lives in `variables.tf` | none (existing) |
| 4 | `VariableBlock.AutoFix`: re-order `type → default → description → nullable → sensitive` | `reorder_attributes` + `for_each = data.variable.all.result` | `reorder_attributes` |
| 5 | `VariableBlock.AutoFix`: drop `nullable = true` (default) | `remove_block_element` + `for_each` filtered on `try(v.nullable, "") == "true"` | none (existing) |
| 6 | `VariableBlock.AutoFix`: drop `sensitive = false` (default) | same pattern as #5 | none (existing) |
| 7 | `OutputsFile.AutoFix`: sort output blocks alphabetically inside `outputs.tf` | `sort_blocks_in_file` | covered by #1 |
| 8 | `HclFile.AutoFix`: move stray `output` blocks into `outputs.tf` | `move_block` + `for_each = data.output.all.result` | none (existing) |
| 9 | `OutputsFile.AutoFix`: move non-output blocks out of `outputs.tf` | `move_block` + `for_each` over each non-output kind | none (existing) |
| 10 | `OutputBlock.AutoFix`: re-order attributes alphabetically by name | `reorder_attributes` with an alphabetical list | covered by #4 |
| 11 | `OutputBlock.AutoFix`: drop `sensitive = false` | `remove_block_element` + `for_each` filter | covered by #5 |
| 12 | `ModuleBlock.AutoFix`: head meta args ordered `for_each/count → source → version → providers` | `reorder_attributes` + `head_attributes` + `for_each` over module blocks | covered by #4 + `data "module"` |
| 13 | `ModuleBlock.AutoFix`: tail meta args ordered `depends_on` last | `reorder_attributes` + `tail_attributes` | covered by #4 |
| 14 | `ModuleBlock.AutoFix`: required inputs (no default in source module) alphabetical, then optional alphabetical | **DEFERRED** — needs source-module variable introspection | future enhancement (see below) |
| 15 | `MovedBlock.AutoFix`: order `from → to` | `reorder_attributes` + `for_each` over moved blocks | covered by #4 + `data "moved"` |
| 16 | `RemovedBlock.AutoFix`: `from → lifecycle → provisioner` layout | **DEFERRED** — nested-block ordering not currently expressible | future enhancement |
| 17 | `LocalsBlock.AutoFix`: sort locals attributes alphabetically | **EXCLUDED** per user requirement | n/a |
| 18 | `TerraformBlock.AutoFix`: layout + sort providers inside `required_providers` | **DEFERRED** — nested-block re-ordering / sub-block targeting not currently expressible | future enhancement |
| 19 | `ResourceBlock.AutoFix` / `BuildBlockWithSchema`: head meta `for_each/count → provider`, tail meta `lifecycle → depends_on` | `reorder_attributes` + `head_attributes` / `tail_attributes` | covered by #4 |
| 20 | `ResourceBlock.AutoFix`: required-before-optional body attribute sort using provider schema | Optional, in scope: HCL pre-computed list using `data.provider_schema` → `reorder_attributes` (limited to root attributes; nested block re-ordering deferred) | example provided |
| 21 | `terraform init` prerequisite for module / schema lookups | Out of scope — handled by the pre-commit pipeline, not by mapotf | n/a |
| 22 | `NewBlockTransform.Format()` currently calls `avmfix.BuildVariableBlock` and `avmfix.BuildBlockWithSchema(&hcl.File{})` | Replace with internal logic: for `variable`, sort attrs by the variableAttributePriorities map; for `resource`/`data`, no-op (current effective behaviour) | rewrite `Format()` |

Items 1–13, 15, 19, 22 are sufficient to drop `avmfix -folder .` from the AVM pre-commit
pipeline today. Items 14, 16, 18 are explicitly listed as deferred so the user knows the
exact regression surface; each can later be addressed by a small follow-up without
re-litigating the design.

---

## New transform 1 — `reorder_attributes`

### Responsibility

Re-order the attributes of exactly one block. No file movement. No removal. Does nothing to
nested blocks.

### Schema

```hcl
transform "reorder_attributes" "<name>" {
  target_block_address = "<address>"                    # required
  head_attributes      = ["attr1", "attr2", ...]        # optional, default []
  tail_attributes      = ["attr_last", ...]             # optional, default []
}
```

### Semantics

Given a target block whose current attribute order is `[a, b, c, d, e]`:
1. Walk `head_attributes`; for each name present on the block, write it first in the given order.
2. Walk the **original** attribute order and write every attribute that is in neither list.
3. Walk `tail_attributes`; for each name present on the block, write it last in the given order.

Names in either list that are not present on the block are silently skipped. The same name
appearing in both `head_attributes` and `tail_attributes` is a configuration error.

### Implementation sketch

`pkg/transform_reorder_attributes.go`:
- Embed `*golden.BaseBlock` and `*BaseTransform`.
- `Type() string` returns `"reorder_attributes"`.
- `Apply()`:
  1. Look up `block` via `cfg.RootBlock(target_block_address)`.
  2. Snapshot the existing `block.WriteBody().Attributes()` (a map) and the original
     declaration order using `block.Attributes` (which preserves source order on the
     `hclsyntax` side via `attributesByLines`-style iteration). Each `*hclwrite.Attribute`
     keeps its expression tokens.
  3. Compute the new sequence: `head` (filtered to present names) → `middle` (original-order
     names not in head or tail) → `tail` (filtered to present names).
  4. Remove every attribute from `block.WriteBody()` via `RemoveAttribute(name)`.
  5. Re-add them in the new order via `block.WriteBody().SetAttributeRaw(name, expr.BuildTokens(nil))`.
  6. Nested blocks under the body are untouched (they remain at their current position;
     mutating their order is out of scope for v1).

---

## New transform 2 — `sort_blocks_in_file`

### Responsibility

Ensure a set of named blocks lives in a given file, in the given order. Blocks already in
the file that are not in the list are left in place at the top, preserving their original
relative order.

### Schema

```hcl
transform "sort_blocks_in_file" "<name>" {
  file_name     = "variables.tf"                                       # required, .tf
  desired_order = ["variable.location", "variable.name", "variable.tags"]   # required
}
```

### Semantics

For each address in `desired_order`:
1. Look the block up in any file via `cfg.RootBlock(address)`.
   - If not found anywhere, return an error naming the missing address.
   - If found and already in `file_name`, mark it for re-positioning.
   - If found in another file, mark it for cross-file re-positioning.
2. After all addresses are resolved:
   - For each block flagged in step 1, remove it from its current `hclwrite.File` body via
     `cfg.module.RemoveBlock(writeBlock)`.
   - Append the blocks to `file_name` in `desired_order` via `cfg.AddBlock(file_name, writeBlock)`.

Blocks already present in `file_name` that are NOT in `desired_order` are not removed and
not re-positioned — they remain at the top of the file, in original order.

This **does** combine "consolidate into a file" with "order within the file", because both
are facets of the same single concern: deterministic block placement at a file level. It is
deliberately separated from `reorder_attributes`, which operates on a different abstraction
level (attribute layout inside one block).

### Implementation sketch

`pkg/transform_sort_blocks_in_file.go`:
- Embed `*golden.BaseBlock` and `*BaseTransform`.
- `Type() string` returns `"sort_blocks_in_file"`.
- `Apply()`:
  1. Validate `FileName` ends with `.tf` (struct tag handles this).
  2. For each `addr` in `DesiredOrder`, fetch `block := cfg.RootBlock(addr)`; error if nil.
  3. Build a slice of `*hclwrite.Block` references in `desired_order` order.
  4. Call `cfg.module.RemoveBlock(b)` for each; this removes from whichever file holds them.
  5. Call `cfg.AddBlock(file_name, b)` for each, in order.
  6. `SaveToDisk` (called at the end of `Apply()` in the plan loop) handles writing the
     new file if it was created by `AddBlock`.

### Edge cases the spec covers

- **Block in `desired_order` is already in `file_name` at the right index** — still
  removed + re-appended; idempotent in effect.
- **`desired_order` lists an address whose block doesn't exist** — error with the address.
  This is preferred to silent skipping because the order is almost always computed from a
  data source; if it disappears, the user should know.
- **`file_name` doesn't exist yet** — `cfg.AddBlock` creates a new `hclwrite.File`; the
  module save loop creates the file on disk with a `.new` marker (existing behaviour).

---

## New data source 1 — `data "module"`

Modelled directly on `data "resource"`. Backing struct in `pkg/data_module.go`:

```go
type DataModuleBlock struct {
    *BaseData
    *golden.BaseBlock
    Source string    `hcl:"source,optional"`     // optional filter
    Result cty.Value `attribute:"result"`
}

func (d *DataModuleBlock) Type() string { return "module" }
```

`ExecuteDuringPlan()` iterates `cfg.ModuleBlocks()`, filters by `Source` if non-empty, and
emits `result = map[module_name] => block.EvalContext()`.

Block label is the module name (`module.<name>`), so `result.<name>` gives the block's
attributes (source, version, providers, depends_on, plus all module inputs) as strings.

## New data source 2 — `data "moved"`

The `moved` block type is currently not loaded at all (`pkg/terraform/module.go`'s
`wantedTypes` map does not include `moved`). To support it:

1. Add `"moved": func(m *Module) *[]*RootBlock { return &m.MovedBlocks }` to `wantedTypes`.
2. Add `MovedBlocks []*RootBlock` to `Module` and include it in `Blocks()`.
3. Expose it on `MetaProgrammingTFConfig` (`movedBlocks map[string]*terraform.RootBlock`,
   accessor `MovedBlocks()`, and a `moved.` branch in `RootBlock(address)`).
4. New `pkg/data_moved.go` modelled on `data "output"` (singular label = the source `from`
   address as a string, but moved blocks have no label so we synthesise one — e.g. the
   block's source position, exposed as `mptf.range.start_line`-based key). The simplest
   key is an integer index `0`, `1`, ... since `moved` blocks have no natural identifier
   and the user iterates them all anyway.

The above plumbing is small (~80 lines + tests) and unblocks #15 in the mapping table.

(`data "removed"` is intentionally omitted because item #16 is deferred — adding the data
source without the nested-block transform would be misleading.)

---

## Modify existing — drop avmfix from `transform_new_block.go`

`pkg/transform_new_block.go` lines 107–129 currently call:
```go
avmfix.BuildBlockWithSchema(avmBlock, &hcl.File{})   // resource / data
avmfix.BuildVariableBlock(&hcl.File{}, avmBlock)     // variable
```

For `resource` / `data`, the call passes an empty schema, so it is effectively a no-op
already; replace with `return block, nil`.

For `variable`, port the small piece of logic from `avmfix/pkg/variables.go`:

```go
var variableAttributePriorities = map[string]int{
    "type":        0,
    "default":     1,
    "description": 2,
    "nullable":    3,
    "sensitive":   4,
}
```

Sort attributes by priority (unknown attrs preserved at end), and drop `nullable = true` /
`sensitive = false` literals using the same primitive (`hclsyntax.LiteralValueExpr` literal
check). This is ~40 lines of self-contained code; once landed, the
`github.com/lonegunmanb/avmfix` import is removed and `go mod tidy` purges it from
`go.mod` and `go.sum`.

---

## Composition examples

These examples are what an AVM module author would actually write in
`mapotf-configs/pre-commit/*.mptf.hcl`. They are pure declarative HCL — no Go.

### 1. Variables to AVM spec

```hcl
data "variable" "all" {}

locals {
  vars           = data.variable.all.result
  required_names = sort([for n, v in local.vars : n if !contains(keys(v), "default")])
  optional_names = sort([for n, v in local.vars : n if  contains(keys(v), "default")])
  ordered_vars   = concat(local.required_names, local.optional_names)
}

# Re-order attributes inside every variable block
transform "reorder_attributes" "var_attrs" {
  for_each             = local.vars
  target_block_address = "variable.${each.key}"
  head_attributes      = ["type", "default", "description", "nullable", "sensitive"]
}

# Drop redundant nullable = true
transform "remove_block_element" "drop_nullable_true" {
  for_each             = { for n, v in local.vars : n => v if try(v.nullable, "") == "true" }
  target_block_address = "variable.${each.key}"
  paths                = ["nullable"]
}

# Drop redundant sensitive = false
transform "remove_block_element" "drop_sensitive_false" {
  for_each             = { for n, v in local.vars : n => v if try(v.sensitive, "") == "false" }
  target_block_address = "variable.${each.key}"
  paths                = ["sensitive"]
}

# Consolidate + order all variable blocks inside variables.tf
transform "sort_blocks_in_file" "variables_tf" {
  file_name     = "variables.tf"
  desired_order = [for n in local.ordered_vars : "variable.${n}"]
}
```

**Before** (a single misplaced, badly-formatted variable in `main.tf`):
```hcl
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

**After** (in `variables.tf`, required first, then alphabetical; attributes re-ordered;
redundant defaults removed):
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

### 2. Outputs to AVM spec

```hcl
data "output" "all" {}

locals {
  outs          = data.output.all.result
  ordered_outs  = sort(keys(local.outs))
}

transform "reorder_attributes" "output_attrs" {
  for_each             = local.outs
  target_block_address = "output.${each.key}"
  # AVM spec for outputs: known attrs alphabetical; unlisted attrs preserved at end
  head_attributes      = ["depends_on", "description", "sensitive", "value"]
}

transform "remove_block_element" "drop_output_sensitive_false" {
  for_each             = { for n, v in local.outs : n => v if try(v.sensitive, "") == "false" }
  target_block_address = "output.${each.key}"
  paths                = ["sensitive"]
}

transform "sort_blocks_in_file" "outputs_tf" {
  file_name     = "outputs.tf"
  desired_order = [for n in local.ordered_outs : "output.${n}"]
}
```

### 3. Move non-variable blocks out of variables.tf

avmfix moves any non-`variable` block out of `variables.tf` into `main.tf`. Same outcome,
declaratively:

```hcl
data "resource" "all" {}
data "data"     "all" {}
data "module"   "all" {}
data "output"   "all" {}

locals {
  in_variables_tf = {
    for addr, b in merge(
      { for t, rs in data.resource.all.result : "resource.${t}.${k}" => v
        for k, v in rs },
      { for t, ds in data.data.all.result : "data.${t}.${k}" => v
        for k, v in ds },
      { for k, v in data.module.all.result : "module.${k}" => v },
      { for k, v in data.output.all.result : "output.${k}" => v },
    ) : addr => b
    if b.mptf.range.file_name == "variables.tf"
  }
}

transform "move_block" "out_of_variables_tf" {
  for_each             = local.in_variables_tf
  target_block_address = each.key
  file_name            = "main.tf"
}
```

(The double-comprehension `for t, rs in ... : for k, v in rs` is illustrative; in practice
the same flattening pattern used by `example/tracing_tags/main.mptf.hcl` is cleaner.)

### 4. Module blocks — basic meta-arg ordering

```hcl
data "module" "all" {}

transform "reorder_attributes" "module_meta" {
  for_each             = data.module.all.result
  target_block_address = "module.${each.key}"
  head_attributes      = ["for_each", "count", "source", "version", "providers"]
  tail_attributes      = ["depends_on"]
}
```

Module inputs (the body of each module call) remain in their original order. The
"required-before-optional inputs" sort is in the **deferred** column because it requires
introspecting the source module's `variables.tf` to know which inputs are required.

### 5. moved blocks

```hcl
data "moved" "all" {}

transform "reorder_attributes" "moved_attrs" {
  for_each             = data.moved.all.result
  target_block_address = "moved.${each.key}"   # key is a synthetic index 0,1,...
  head_attributes      = ["from", "to"]
}
```

### 6. Resource/data with provider schema (optional, in scope)

avmfix uses `terraform providers schema -json` to know which attributes of, say,
`azurerm_resource_group` are required vs optional, then sorts them required-alphabetical then
optional-alphabetical. mapotf already exposes the same schema via `data "provider_schema"`,
so the same outcome is composable without a new transform:

```hcl
data "provider_schema" "azurerm" {
  provider_source  = "hashicorp/azurerm"
  provider_version = "~> 4.0"
}

data "resource" "all" {}

locals {
  res_addrs = flatten([
    for t, rs in data.resource.all.result : [for k, _ in rs : "resource.${t}.${k}"]
  ])
  required_attrs = {
    for t in keys(data.resource.all.result) :
      t => sort([for n, a in data.provider_schema.azurerm.resources[t].block.attributes : n if a.required])
  }
  optional_attrs = {
    for t in keys(data.resource.all.result) :
      t => sort([for n, a in data.provider_schema.azurerm.resources[t].block.attributes : n if !a.required && !a.computed])
  }
}

transform "reorder_attributes" "resource_attrs" {
  for_each             = { for addr in local.res_addrs : addr => addr }
  target_block_address = each.value
  head_attributes      = concat(
    ["for_each", "count", "provider"],
    local.required_attrs[split(".", each.value)[1]],
    local.optional_attrs[split(".", each.value)[1]],
  )
  tail_attributes      = ["lifecycle", "depends_on"]
}
```

Limitations of this approach (vs avmfix's full implementation):
- Only top-level resource attributes are re-ordered. Nested block re-ordering inside a
  resource (e.g. `network_interface { ... }` within `azurerm_virtual_machine`) requires
  addressing nested blocks, which is out of scope for v1.
- `meta`-only blocks like `dynamic { ... }` are left as-is.

AVM pre-commit does not need to enable this until/unless the modules want it.

---

## Explicitly deferred capabilities

| # | Capability | Why deferred | Path forward |
|---|---|---|---|
| 14 | Module input required-before-optional ordering | Needs to read the source module's `variables.tf` (avmfix does this via `tfconfig.LoadModule`). mapotf currently only models the calling module. | Add `data "module_source_variables"` that takes a `source` string and exposes the target module's variable list; user supplies the sort to `reorder_attributes`. |
| 16 | `removed` block layout | Nested blocks (`lifecycle`, `provisioner`) need to be re-ordered along with attributes; `reorder_attributes` v1 only handles attributes. | Future `reorder_block_elements` (combined attrs + nested blocks) — out of scope here. |
| 18 | Terraform block layout + providers sort | Requires addressing a nested block (`required_providers`) inside `terraform`. mapotf's `target_block_address` is for root blocks. | Future: extend addressing OR a dedicated `sort_terraform_block` transform. |
| 17 | `locals` block sort | Excluded per user requirement. | n/a |

None of the deferred items block the AVM pre-commit migration.

---

## Files added / modified

| File | Action |
|---|---|
| `pkg/transform_reorder_attributes.go` | NEW — transform implementation |
| `pkg/transform_reorder_attributes_test.go` | NEW — unit tests |
| `pkg/transform_sort_blocks_in_file.go` | NEW — transform implementation |
| `pkg/transform_sort_blocks_in_file_test.go` | NEW — unit tests |
| `pkg/data_module.go` | NEW — `data "module"` |
| `pkg/data_module_test.go` | NEW |
| `pkg/data_moved.go` | NEW — `data "moved"` |
| `pkg/data_moved_test.go` | NEW |
| `pkg/terraform/module.go` | MODIFY — add `moved` to `wantedTypes`, add `MovedBlocks` field, include in `Blocks()` |
| `pkg/mptf_config.go` | MODIFY — add `movedBlocks` map + `MovedBlocks()` accessor + `moved.` branch in `RootBlock()` |
| `pkg/transform_new_block.go` | MODIFY — replace avmfix calls in `Format()` with internal logic |
| `pkg/register_blocks.go` (or wherever golden registration lives) | MODIFY — register new transforms + data sources |
| `go.mod` / `go.sum` | MODIFY via `go mod tidy` — drop `github.com/lonegunmanb/avmfix` and its transitive deps not used elsewhere |
| `.goreleaser.yaml` | NEW — 6-target build (linux/darwin/windows × amd64/arm64), zip on Windows, tar.gz elsewhere, checksums, no signing |
| `.github/workflows/release.yml` | NEW — triggered on `v*.*.*` tag push, runs goreleaser |

### Implementation order (dependency order)

1. `pkg/transform_reorder_attributes.go` + test (zero new dependencies)
2. `pkg/transform_sort_blocks_in_file.go` + test
3. `data "module"` + test
4. `moved` block plumbing in `pkg/terraform/module.go` + `pkg/mptf_config.go`
5. `data "moved"` + test
6. Replace avmfix in `pkg/transform_new_block.go` + extend tests for variable formatting parity
7. `go mod tidy` — verify `lonegunmanb/avmfix` is gone from `go.mod` and `go.sum`
8. Full `go test ./...` to catch regressions
9. `.goreleaser.yaml`
10. `.github/workflows/release.yml`

---

## goreleaser configuration

`.goreleaser.yaml`:
```yaml
version: 2
project_name: mapotf
before:
  hooks:
    - go mod tidy
builds:
  - id: mapotf
    main: ./
    binary: mapotf
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}
archives:
  - id: mapotf
    name_template: '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}'
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    files: [LICENSE, readme.md]
checksum:
  name_template: 'checksums.txt'
snapshot:
  version_template: '{{ incpatch .Version }}-snapshot'
changelog:
  use: github
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
release:
  draft: false
  prerelease: auto
```

`.github/workflows/release.yml`:
```yaml
name: release
on:
  push:
    tags: ['v*.*.*']
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

No signing, no notarisation, no homebrew, no scoop, no `nfpms` (deb/rpm) — unsigned binaries
in `.tar.gz` and `.zip` archives only, per user requirement.

---

## Follow-up in `avm-terraform-governance`

(Separate PR in that repo, listed here for context only.)

`porch-configs/pre-commit.porch.yaml`:
- Remove the `avmfix` `command_group` entirely
- Remove the three serial avmfix invocations on root, `modules/*`, and `examples/*`
- Add three new files under `mapotf-configs/pre-commit/`:
  - `sort_variables.mptf.hcl` — example #1 above
  - `sort_outputs.mptf.hcl` — example #2 above
  - `move_misplaced_blocks.mptf.hcl` — example #3 above
  - `order_module_attrs.mptf.hcl` — example #4 above (optional, only meaningful in `examples/*`)
- These are picked up automatically by the existing `mapotf transform` invocation; no new
  command needed in the porch config.

---

## Todos (implementation tracking)

| id | title |
|---|---|
| reorder-attributes-transform   | Implement `reorder_attributes` in `pkg/transform_reorder_attributes.go` + register |
| reorder-attributes-test        | Unit tests for `reorder_attributes` (head-only, head+tail, missing attrs, unlisted preservation) |
| sort-blocks-in-file-transform  | Implement `sort_blocks_in_file` in `pkg/transform_sort_blocks_in_file.go` + register |
| sort-blocks-in-file-test       | Unit tests (same-file reorder, cross-file pull-in, missing address error, unlisted preserved at top) |
| data-module                    | Implement `data "module"` in `pkg/data_module.go` + register + test |
| moved-block-plumbing           | Add `moved` to `wantedTypes`; `MovedBlocks` on `Module`; `MovedBlocks()` + `moved.` on `MetaProgrammingTFConfig` |
| data-moved                     | Implement `data "moved"` in `pkg/data_moved.go` + register + test |
| replace-avmfix-inline          | Rewrite `Format()` in `pkg/transform_new_block.go` with internal logic; extend tests for variable parity |
| remove-avmfix-dep              | `go mod tidy`; verify `lonegunmanb/avmfix` is purged from `go.mod` + `go.sum`; full `go test ./...` |
| goreleaser-config              | Add `.goreleaser.yaml` |
| release-workflow               | Add `.github/workflows/release.yml` |
| governance-followup-note       | Capture the AVM pre-commit changes in a follow-up issue (separate repo) |

Dependencies:
- `reorder-attributes-test` depends on `reorder-attributes-transform`
- `sort-blocks-in-file-test` depends on `sort-blocks-in-file-transform`
- `data-moved` depends on `moved-block-plumbing`
- `remove-avmfix-dep` depends on `replace-avmfix-inline`
- `release-workflow` depends on `goreleaser-config`

---

## Decisions made (defaults the plan chose)

These were the open questions; defaults are picked but each is easy to revisit:

- `reorder_attributes` uses `head_attributes` + `tail_attributes` (not a single `order` list)
  so the very common "X first / Y last, rest preserved" case is expressible without a
  sentinel like `"*"`.
- `sort_blocks_in_file` errors on a missing address rather than silently skipping, because
  the address list is almost always computed from a data source and silent skips would
  hide drift.
- Blocks already in the target file but not listed in `desired_order` are left untouched at
  the top of the file (not removed, not re-positioned). This matches avmfix's behaviour
  where stray blocks are moved elsewhere by separate logic and the sort only handles the
  type it cares about.
- Schema-driven sorting of resource/data body attributes is **in scope** as a composable
  recipe using existing `data "provider_schema"`, but only for top-level attributes.
  Nested-block re-ordering inside a resource is deferred to a future transform.
- `data "removed"` is intentionally not added until the matching nested-block transform
  exists, so the data source doesn't ship without a use case.
