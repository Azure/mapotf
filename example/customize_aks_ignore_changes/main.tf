variable "resource_group_name" {
  type    = string
  default = "aks_test"
}

provider "azurerm" {
  features {}
}

resource "random_pet" "this" {}

resource "azurerm_resource_group" "rg" {
  location = "eastus"
  name     = "${var.resource_group_name}-${random_pet.this.id}"
}

module "aks" {
  source  = "Azure/aks/azurerm"
  version = "9.1.0"

  cluster_name        = "aks-test"
  prefix              = "akstest"
  resource_group_name = azurerm_resource_group.rg.name
  rbac_aad            = false
}