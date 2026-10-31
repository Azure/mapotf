# Data "check" Block

`data "check"` reflects top-level Terraform `check` blocks. Its `result` object
is keyed by check label. Each value contains the usual `mptf` metadata and
nested blocks, including the list of `assert` blocks and their `condition` and
`error_message` values. Expressions follow the existing reflection contract:
literals are decoded, while expressions that cannot be evaluated without a
context retain their source text.

Each check's `mptf.references` is a source-ordered list of references from all
attribute expressions in that check, including nested blocks. HCL parses these
references; comments and literal text are not references. Each item contains:

- `root`: the traversal's root name, such as `var` or `local`.
- `traversal`: the exact source text of the traversal.
- `steps`: a list of `{ kind, value, value_type }` objects after the root.
  `kind` is `attribute` or `index`; `value_type` distinguishes string and
  numeric indices. Attribute values have type `string`.
- `range`: `file_name`, `start_line`, `start_column`, `end_line`, and
  `end_column`, using the same source-range convention as other metadata.

References are the free traversals reported by HCL. Computed indices and splats
can yield a static prefix and separate references for their subexpressions;
they are not evaluated. In particular, a bare `var` reference may represent a
computed variable index whose target cannot be determined statically. Local
references are reported as `local.*`, so callers must follow local definitions
when determining transitive dependencies.

```terraform
data "check" "all" {}

locals {
  checks = data.check.all.result
}
```

Check addresses, such as `check.example`, can also be used with existing
transforms.
