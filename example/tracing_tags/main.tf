resource "azurerm_resource_group" "this" {
  location = "West US"
  name     = "example-resources"
}

resource "azurerm_storage_account" "this" {
  name                     = "storageaccountname"
  resource_group_name      = azurerm_resource_group.this.name
  location                 = azurerm_resource_group.this.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags = {
    env = "prod"
  }
}

resource "azurerm_subnet" "this" {
  address_prefixes = []
  name                 = ""
  resource_group_name  = ""
  virtual_network_name = ""
}

