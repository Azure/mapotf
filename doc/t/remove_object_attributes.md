# `remove_object_attributes` Transform Block

Removes exact entries from a literal object without rebuilding retained values or their aliases/comments.

```hcl
transform "remove_object_attributes" telemetry {
  target_block_address   = "module.example"
  attribute_path         = ["providers"]
  keys                   = ["modtm", "random.telemetry"]
  remove_empty_attribute = true
}
```

| Argument | Meaning |
| --- | --- |
| `target_block_address` | Required root-block address. A missing block is an error. |
| `attribute_path` | Required nonempty list: top-level attribute name followed by zero or more literal object keys. Does not traverse nested blocks. |
| `keys` | Required list of exact keys to remove from the selected object. An empty list is permitted. |
| `remove_empty_attribute` | Optional bool, default `false`. Remove the top-level attribute if its object becomes or already is empty. Only valid with a one-segment `attribute_path`. |

`["settings", "a.b"]` selects the literal `a.b` object member inside `settings`; it does not select `settings.a.b`. Similarly `keys = ["azapi.owner"]` removes that exact quoted key, not `azapi` or a nested `owner` member. Key spelling is decoded, so escaped quoted keys match their decoded names.

Missing attributes, missing path members and missing removal keys are no-ops. Existing objects along the selected path must have static, unique keys. References, calls, conditionals, comprehensions and computed/duplicate keys at those objects are errors, even when the requested removal keys are absent. Retained value expressions may be arbitrary HCL.

All validation and syntax checks occur before changing the target attribute. A failing plan does not save Terraform files. Deletion removes selected key/value expressions and their following commas only; all comments outside those expressions are retained, including standalone/orphaned comments associated with removed entries. Retained expression bytes are not reconstructed; normal MaPoTF file formatting still applies.

With the default, deleting every entry retains `{}` (and any external comments). Explicit empty-attribute removal also removes attribute-owned comments. Repeated removal is idempotent. Order multiple transforms editing the same attribute with `depends_on`.

Use [`object_entries`](../functions/object_entries.md) for profile decisions and `update_in_place` with `merge_object_attributes = true` for additive/replacement edits. The merge is shallow: patch a provider requirement within `required_providers`, rather than rebuilding its containing object, to retain unmentioned source, version and configuration aliases.

The existing `remove_block_element` transform is unchanged: its dotted paths traverse nested blocks/attributes, **not object entries**. Do not use it to delete selected entries of a module's `providers` object.
