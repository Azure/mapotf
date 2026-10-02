# Data `"test_file"` block

Select one existing Terraform test file, relative to its owning module root:

```powershell
mapotf transform --tf-dir C:\src\module --test-file tests\unit\basic.tftest.hcl --mptf-dir C:\rules
```

Only the selected file is read for transformation, backed up, and written. Ordinary `.tf` discovery is unchanged without `--test-file`. Selection cannot be combined with `--recursive` or commands that execute Terraform. Paths outside the owning module and writes to another file are rejected. Use the same `--tf-dir` and `--test-file` with `reset` or `clean-backup` to restore or discard just that file's backup.

```hcl
data "test_file" "this" {}
```

## Result

`result` contains:

| Field | Value |
| --- | --- |
| `filename` | Selected relative filename, using the host's path separators. |
| `module_dir` | Absolute owning module directory. |
| `variables` | Global `variables` block, or `null` when absent. |
| `runs` | Object keyed by run name. |
| `mock_providers` | Object keyed by provider name or `name.alias`. |
| `providers` | Authored real provider blocks, keyed by name or `name.alias`. |
| `run_modules` | Object keyed by run name; each value contains `kind`, `source`, and `dir`. |

Block attributes use typed literal values; expressions requiring evaluation remain HCL strings. Nested blocks are tuples, without coercing different objects to a shared type. A run's variables are at `runs.<name>.variables[0]`; the `variables` property is absent when that run has no variables block. Assertions, provider mappings, module blocks, and `expect_failures` remain available.

Every block has `mptf.block_type`, `mptf.block_labels`, `mptf.attributes`, `mptf.is_empty`, and source-range reflection. `mptf.attributes` contains exactly the authored attributes, including a possible input named `mptf`. Check its keys to distinguish a missing input from an explicit `null`, expression, or other value. `is_empty` means no attributes or nested blocks; an alias, `source`, custom defaults, or overrides makes a mock nonempty.

Root block addresses are `variables`, `run.<name>`, `mock_provider.<name>[.<alias>]`, and `provider.<name>[.<alias>]`. Use `mptf.block_address` rather than constructing them. Addresses are local to the selected file: identical names in other files do not collide.

`run_modules` distinguishes:

| `kind` | `source` | `dir` |
| --- | --- | --- |
| `root` | `null` (no module block) | Owning module root. |
| `local` | Authored local source string. | Resolved absolute directory. |
| `remote` | Authored remote source string. | `null`; never downloaded by this data source. |

Local sources are relative to the owning module root, **not the test file's directory**. Missing local directories, duplicate run/provider identities, multiple global or per-run variables/module blocks, and missing/dynamic/non-string module sources are errors. A present but invalid module block never defaults to the root.

## Inspect declarations without editing

Reuse `module_source` for known local targets:

```hcl
data "test_file" "this" {}

data "module_source" "targets" {
  for_each = {
    for name, target in data.test_file.this.result.run_modules : name => target
    if target.kind != "remote"
  }
  source = each.value.dir
}
```

```powershell
mapotf debug --tf-dir C:\src\module --test-file tests\unit\basic.tftest.hcl --mptf-dir C:\inspection `
  --eval '{ test = data.test_file.this.result, modules = { for name, target in data.module_source.targets : name => target.variables } }'
```

`--eval` writes exactly one JSON value and a newline to stdout. Parse/evaluation/encoding failures return a nonzero exit code. It runs the configuration's plan, but never applies transforms or creates backups. Local `module_source` inspection does not invoke Terraform or use the network. Without `--eval`, debug retains its interactive mode.

`--mptf-var` accepts one `name=value` assignment per occurrence. Repeat the flag for additional assignments, for example `--mptf-var 'targets=["C:\\src\\module","C:\\src\\module\\modules\\child"]' --mptf-var 'enabled=true'`.

`--mptf-var-file` accepts one filename per occurrence. Repeat it for additional files: `--mptf-var-file C:\rules\base.mptfvars --mptf-var-file C:\rules\overrides.mptfvars`.

Each value is passed intact; commas inside HCL collections, strings, or filenames are not separators. CSV-grouping multiple assignments or filenames into one flag occurrence is not supported.

## Conditional edits

This example adds a run input only where neither the global nor per-run variables supply it:

```hcl
data "test_file" "this" {}

transform "update_in_place" "missing_input" {
  for_each = {
    for name, run in data.test_file.this.result.runs : name => run
    if !contains(keys(try(run.variables[0].mptf.attributes, {})), "location") &&
       !contains(keys(try(data.test_file.this.result.variables.mptf.attributes, {})), "location")
  }
  target_block_address = each.value.mptf.block_address
  asraw {
    variables { location = "eastus" }
  }
}
```

The patch merges attributes into an existing variables block or adds that nested block when absent. Unrelated inputs, expressions, module targets, assertions, and opt-outs are preserved. This example does not check whether a target declares `location`; use declaration inspection and a narrower predicate when that matters. MaPoTF adds no inputs or mocks automatically.

To create a missing global block:

```hcl
transform "new_block" "global_variables" {
  for_each       = data.test_file.this.result.variables == null ? { create = true } : {}
  filename       = data.test_file.this.result.filename
  new_block_type = "variables"
  asraw { location = "eastus" }
}
```

`update_in_place`, `append_block_body`, `remove_block`, element edits, and ordering transforms use the existing interfaces. File-targeted transforms must name the selected file. `ensure_local` is unsupported because test files do not declare locals. Data reflects the file before applying the plan; use a subsequent invocation to query newly added blocks.

HCL parsing preserves expressions, comments, templates, and heredocs. Compact nonempty blocks are expanded to multiline form when written. This is syntax-aware editing, not full Terraform test validation or execution. Top-level `test` and override blocks are preserved but not exposed as transform targets; unsupported block shapes fail explicitly.
