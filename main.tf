data resource k8s {
  type = "azurerm_kubernetes_cluster"
}

fix merge k8s_ignore_change {
  for_each = data.resource.k8s
  lifecycle {
    ignore_change = merge(try(each.value.lifecycle[0].ignore_change, []), [microsoft_defender[0].log_analytics_workspace_id])
  }
}