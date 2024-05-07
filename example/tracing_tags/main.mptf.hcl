data "resource" azurerm_cognitive_account {
  resource_type = "azurerm_cognitive_account"
}

transform "update_in_place" aks_ignore_changes {
  for_each = try(data.resource.azurerm_cognitive_account.result.azurerm_cognitive_account, {})
  target_block_address = each.value.mptf.block_address
  asstring {
    tags = <<-TAGS
          merge({
                  file = "${each.value.mptf.range.file_name}"
                  block = "${each.value.mptf.terraform_address}"
                }, ${try(each.value.tags, "{}")})
    TAGS
  }
}