data "resource" aks {
  resource_type = "azurerm_kubernetes_cluster"
}

transform "update_in_place" aks_ignore_changes {
  for_each = try(data.resource.aks.result.azurerm_kubernetes_cluster, {})
  target_block_address = each.value.mptf.block_address
  asstring {
    lifecycle {
      ignore_changes = "[\nmicrosoft_defender[0].log_analytics_workspace_id, ${trimprefix(try(each.value.lifecycle.0.ignore_changes, "[\n]"), "[")}"
    }
  }
}