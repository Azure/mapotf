data "resource" azurerm_kubernetes_cluster {
  resource_type = "azurerm_kubernetes_cluster"
}

transform "update_in_place" tracing_tags {
  for_each = try(data.resource.azurerm_kubernetes_cluster.result.azurerm_kubernetes_cluster, {})
  target_block_address = each.value.mptf.block_address
  asstring {
    tags = <<-TAGS
          merge({
                  file = "${each.value.mptf.range.file_name}"
                  block = "${each.value.mptf.terraform_address}"
                  module_source = "${each.value.mptf.module.source}"
                  module_version = "${each.value.mptf.module.version}"
                }, ${try(each.value.tags, "{}")})
    TAGS
  }
}