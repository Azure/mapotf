# Data `"module_source"` block

The `module_source` data source fetches a remote Terraform module by its `source` address (registry shortcut, Git URL, local path, etc.) and exposes the module's declared variables and outputs. The pre-sorted `required_variables` / `optional_variables` lists are intended to feed directly into `reorder_attributes.body_attributes` so every `module.<name>` call site can be re-ordered to match the source module's variable contract.

## Arguments

- `source` (String, Required): The module source, exactly as you would write it in a `module { source = "..." }` block — for example `Azure/naming/azurerm`, `git::https://github.com/Azure/terraform-azurerm-aks.git?ref=v9.0.0`, or `./modules/storage`.
- `version` (String, Optional): A version constraint, only meaningful for sources that support versioning (e.g. registry sources). Same syntax as the `version` argument of a `module` block — for example `~> 0.4`.
- `base_dir` (String, Optional): Directory used to resolve a relative `source` (e.g. `./submod`, `../shared`). When omitted, defaults to the absolute path of the Terraform module mapotf is currently transforming — so a `data "module_source" { source = "../../" }` in `mapotf-configs/*.mptf.hcl` resolves to the same parent module Terraform itself would resolve. Ignored for remote sources (registry shortcuts, Git URLs).

## Attributes

- `variables` (Object): A map keyed by variable name. Each value is an object with:
    - `required` (Bool): `true` when the variable has no `default` in the source module.
    - `type` (String): The HCL type expression as written in the source module's `variable` block (for example `string`, `map(string)`, `object({ name = string })`). May be empty if the source module declares no explicit type.
    - `description` (String): The variable's `description` attribute, or `""` if absent.
    - `sensitive` (Bool): The variable's `sensitive` attribute, or `false` if absent.
    - `default` (String, nullable): String rendering of the variable's `default` value, or `null` for required variables.
- `outputs` (Object): A map keyed by output name. Each value is an object with:
    - `description` (String)
    - `sensitive` (Bool)
- `required_variables` (List of String): Alphabetically-sorted list of variable names that have no `default` in the source module.
- `optional_variables` (List of String): Alphabetically-sorted list of variable names that have a `default` in the source module.

## Example - Order a module call's inputs required-then-optional

```terraform
data "module_source" "naming" {
  source  = "Azure/naming/azurerm"
  version = "~> 0.4"
}

transform "reorder_attributes" "naming_module" {
  target_block_address = "module.naming"
  head_attributes      = ["source", "version", "providers", "for_each", "count"]
  body_attributes = concat(
    data.module_source.naming.required_variables,
    data.module_source.naming.optional_variables,
  )
  foot_attributes = ["depends_on"]
}
```

The body of `module "naming"` is rewritten to emit every required input first (alphabetical), followed by every optional input the call site sets (alphabetical), separated from the head and foot sections by the blank lines that `reorder_attributes` adds when `head_foot_line_breaks` is `true` (the default).

Inputs the call site sets that aren't declared by the source module are not lost — they land at the end of the body, sorted alphabetically by default (or in source order if `sort_body_alphabetically = false`).

## Example - Apply the same ordering to every `module` call

Combine `data "module_source"` with `data "module"` and `for_each` to enforce the contract uniformly across an entire repository:

```terraform
data "module" "all" {}

data "module_source" "naming" {
  source  = "Azure/naming/azurerm"
  version = "~> 0.4"
}

# Look up the right module_source per module call by inspecting the
# module block's `source` attribute. Here we keep it simple with a
# single shared source.
locals {
  module_addresses = { for name, _ in data.module.all.result : name => name }
}

transform "reorder_attributes" "module_inputs" {
  for_each             = local.module_addresses
  target_block_address = "module.${each.key}"
  head_attributes      = ["source", "version", "providers", "for_each", "count"]
  body_attributes = concat(
    data.module_source.naming.required_variables,
    data.module_source.naming.optional_variables,
  )
  foot_attributes = ["depends_on"]
}
```

For a multi-source repository, branch on `data.module.all.result[each.key].source` to pick the right `data "module_source"` block.

## Under the Hood

Mapotf parses the requested module in one of two ways depending on the source:

- **Local sources** (`./`, `../`, absolute paths): resolved against `base_dir` and parsed directly with `terraform-config-inspect`. No `terraform` invocation, no temp folder, no network. This is the path AVM modules' `examples/*` configurations use to point at the parent module under test via `source = "../../"`.
- **Remote sources** (registry shortcuts, Git URLs, etc.): mapotf synthesises a minimal Terraform configuration that calls the requested module in a temporary directory, runs `terraform get` to fetch the module source, then loads the fetched module with `terraform-config-inspect` to read its variable and output declarations. `terraform get` fetches the module only — it does **not** download provider plugins, so `data "module_source"` is faster than `data "provider_schema"` and works without provider credentials. If `terraform get` reports validation errors (e.g. the synthetic wrapper doesn't satisfy the target module's required inputs) but the module body itself was downloaded successfully, mapotf ignores the validation error and reads the downloaded `.tf` files anyway, because `terraform-config-inspect` only needs the module's declarations, not a fully-validated wrapper.

Remote sources containing `//subdirectory` are loaded from the exact module directory recorded in Terraform's `modules/modules.json` manifest, not the repository root. The manifest is read from `TF_DATA_DIR` when set, or `.terraform` otherwise; relative paths are resolved against the temporary Terraform working directory.

## Remote inspection cache

Data blocks reuse variable/output metadata across processes for public `registry.terraform.io` root modules pinned to a bare, stable three-part version, such as `Azure/naming/azurerm` with `version = "0.4.2"`. The first fetch still runs Terraform; only successful downloads whose manifest matches the requested source and version populate the cache. Tolerated Terraform errors never populate it.

Local modules are always reloaded, including between sequential transform profiles. Version constraints (including `= 0.4.2`), prereleases, subdirectories, Git/URL sources and private registries remain uncached. The exported Go `NewTerraformCliModuleSourceFetcher` also retains its uncached, full-module behavior.

| Environment variable | Default | Behavior |
| --- | --- | --- |
| `MAPOTF_MODULE_SOURCE_CACHE` | enabled | `off`, `false`, `0`, `no` or `disabled` bypass reads and writes; `refresh` fetches again and replaces successful entries. |
| `MAPOTF_MODULE_SOURCE_CACHE_DIR` | OS user cache directory, under `mapotf/module-source` | Override the dedicated cache directory, for example under `AVM_HOME`. Keep it private to the current user. |
| `MAPOTF_MODULE_SOURCE_CACHE_TTL` | `168h` | Go duration until an entry expires; zero or a negative duration disables expiry. |
| `MAPOTF_MODULE_SOURCE_CACHE_LOCK_TIMEOUT` | `2m` | Positive Go duration limiting how long to wait for another process populating the same entry. |

Per-entry OS locks prevent duplicate population and are released automatically if a process exits. Waiting honors cancellation; timeout returns an actionable error rather than starting a duplicate download. Invalid duration settings warn and use their defaults. Cache I/O or corrupt-entry failures warn and retry through the normal fetch path; errors from that fetch are not hidden.

Entries are format- and inspector-version-specific, checksum-validated, and replaced through a temporary file while holding the lock. Variable/output metadata must round-trip without changing default types; unsupported legacy default representations stay uncached. Cache hits preserve the data source's existing attributes, including its string rendering of non-null defaults. To invalidate entries, use `refresh` or remove the cache's `.json` files while no transforms are running. Do not remove `.lock` files while processes may be using them.

The cache conservatively bypasses custom Terraform CLI contexts: `TF_CLI_CONFIG_FILE`, `TERRAFORM_CONFIG`, `TF_CLI_ARGS`, `TF_CLI_ARGS_get`, `TF_DATA_DIR`, any nonempty `TF_TOKEN_*`, the default CLI configuration file, or any `.tfrc`/`.tfrc.json` configuration fragment. This includes otherwise benign CLI configuration files. On Windows the default paths come from the OS roaming-app-data folder, as Terraform uses, not an overridden `APPDATA`. Configuration contents and credentials are never read into cache keys or entries. Builds without a versioned inspector dependency also bypass caching.
