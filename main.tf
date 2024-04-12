data resource k8s {
  type = "azurerm_kubernetes_cluster"
}

fix merge k8s_ignore_change {
  for_each = data.resource.k8s
  lifecycle {
    ignore_change = lexexp("[microsoft_defender[0].log_analytics_workspace_id]")
  }
}

data resource k8s_node_pool {
  type = "azurerm_kubernetes_cluster"
}

locals {
  k8s_node_pool_with_for_each = [ for block in data.resource.k8s_node_pool : block if block.for_each_iterator != null ]
}

fix merge k8s_node_pool_ignore_change {
  for_each = local.k8s_node_pool_with_for_each
  lifecycle {
    ignore_change = lexexp("[microsoft_defender[0].log_analytics_workspace_id]")
  }
}

data resource cognitive_account {
  type = "azurerm_cognitive_account"
}

locals {
  cognitive_accounts_with_for_each = [ for block in data.resource.cognitive_account : block if block.for_each_iterator != null ]
}

fix newresource private_endpoint_for_cognitive_account {
  type = "azurerm_private_endpoint"
  name = "this"
  for_each = locals.cognitive_accounts_with_for_each
  # now `each.value` stands for an `azurerm_cognitive_account` block with `for_each`, for example, `azurerm_cognitive_account.this["eastus"]`
  # let's say we have：
  # resource azurerm_cognitive_account this {
  #   for_each = toset["eastus", "westus"]
  #   ...
  #   location = each.value
  # }
  # `each.value.for_each_iterator` stands for `for_each = toset["eastus", "westus"]`
  # exp(each.value.for_each_iterator) stands for `toset["eastus", "westus"]`
  for_each_iterator = exp(each.value.for_each_iterator)
  /* So we got such new resource block:
  resource azurerm_private_endpoint this {
    for_each = toset["eastus", "westus"]
  }
  */
  # `each.value` now is `azurerm_cognitive_account.this`, it's a map, so we need `withindex`
  # `withindex(each.value, each_iterator.value)` now is `azurerm_cognitive_account.this[each.value]`
  location = addr(withindex(each.value, for_each_iterator.value).location)
  # `azurerm_cognitive_account.this[each.value].location` is `location = each.value` defined in `azurerm_cognitive_account` block
  # `addr(withindex(each.value, each_iterator.value).location)` is expression: `azurerm_cognitive_account.this[each.value].location`
}