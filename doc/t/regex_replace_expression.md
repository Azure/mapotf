# `regex_replace_expression` Transform Block

The `regex_replace_expression` transform block is a tool in Mapotf that allows you to replace specific patterns in expressions using regular expressions. This is useful when you need to modify configurations by replacing certain patterns with new values.

## Arguments

- `regex`: This argument specifies the regular expression pattern to match in the expressions. The pattern is a string that follows the syntax of Go's `regexp` package.
- `replacement`: This argument specifies the replacement string for the matched patterns. The replacement string can include references to captured groups from the regular expression.

## Example

Here is an example of how to use the `regex_replace_expression` transform block to replace patterns in expressions:

```terraform
transform "regex_replace_expression" this {
  regex       = "azurerm_kubernetes_cluster\\.(\\s*\\r?\\n\\s*)?(\\w+)(\\[\\s*[^]]+\\s*\\])?(\\.)(\\s*\\r?\\n\\s*)?location"
  replacement = "azurerm_kubernetes_cluster.$${1}$${2}$${3}$${4}$${5}region"
}
```

In this example, the `regex` argument specifies a pattern that matches the `location` attribute of `azurerm_kubernetes_cluster` resources. The `replacement` argument specifies that the matched pattern should be replaced with `region`.

## Detailed Behavior

The `regex_replace_expression` transform block works by traversing all expressions in the Terraform configuration and applying the specified regular expression replacement. The replacement is applied to both attributes and nested blocks.

### Example Scenarios

```terraform
locals {
  azurerm_kubernetes_cluster_location = azurerm_kubernetes_cluster.example[0].location
}
```

After applying the transform:

```terraform
locals {
  azurerm_kubernetes_cluster_location = azurerm_kubernetes_cluster.example[0].region
}
```

In summary, the `regex_replace_expression` transform block is a powerful tool for modifying Terraform configurations by replacing patterns in expressions using regular expressions. This allows for flexible and precise updates to configuration files.