# `object_entries(input)`

Returns a `map(string)` of literal object keys to **unevaluated HCL value-expression text**. Expressions are not resolved against Terraform variables or providers.

```hcl
object_entries("{ azapi = azapi.owner, modtm = modtm }")
# { azapi = "azapi.owner", modtm = "modtm" }

object_entries("{ \"azapi.owner\" = (azapi.owner) }")
# { "azapi.owner" = "(azapi.owner)" }
```

The input can be an expression string or an already-decoded object/map. This supports existing `data.module` attributes, which are typed values when constant and raw expression strings otherwise. Decoded inputs are rendered as HCL first: a decoded string member becomes a **quoted string expression**, not a provider traversal. Empty inputs return an empty map.

```hcl
data "module" calls {}

locals {
  provider_entries = {
    for name, call in data.module.calls.result :
    name => object_entries(try(call.providers, {}))
  }
}
```

Parenthesized objects and literal quoted/escaped keys are supported. Dotted literal keys are not paths. Value expressions retain their original text when the input is an expression string; already-decoded values cannot recover original spelling/comments.

Invalid expressions, nonliteral outer objects (references, function calls, conditionals, comprehensions), computed keys, duplicate decoded keys and unsupported input types return errors. Unknown members in a decoded object/map are rejected. Nested values are opaque expressions; inspection of a nested object requires another call. A missing attribute is distinct from an unsupported expression: only use `try` around the attribute lookup, not around `object_entries`, when errors must stop a profile.

Existing data-source shapes and `tohcl` behavior are unchanged. Use [`remove_object_attributes`](../t/remove_object_attributes.md) for preservation-aware deletion, rather than rebuilding an inspected object.
