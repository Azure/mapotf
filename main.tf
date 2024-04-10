data resource k8s {
  type = "azurerm_kubernetes_cluster"
}


fix merge k8s_ignore_change {
  for_each = data.resource.k8s
  lifecycle {
    ignore_change = merge(try(each.value.lifecycle[0].ignore_change, []), [microsoft_defender[0].log_analytics_workspace_id])
  }
}

data resource sa {
  type = "azurerm_storage_account"
}

fix newresblock private_endpoint_for_sa {
  for_each = data.resource.sa

  type = "azurerm_private_endpoint"
  name = "this"
  location = each.value.location
  // location = azurerm_storage_account.this.location
}