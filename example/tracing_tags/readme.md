## Applying Default Tags with `default_tags` Configuration

This example configuration is designed to ensure that all Terraform-managed AzureRM resources within a project that support tagging are automatically tagged with a predefined set of tags. This is particularly useful for maintaining consistency, facilitating resource management, and adhering to organizational policies regarding resource tagging.

### Purpose

The primary purpose of the `default_tags` configuration is to:

- **Automate Tagging**: Automatically apply a default set of tags (`hello = "world"`) to all resources that support tagging, without the need to manually specify these tags for each resource.
- **Ensure Consistency**: Help maintain a consistent tagging strategy across your infrastructure, which is crucial for resource organization, cost tracking, and access control.
- **Simplify Management**: By applying tags automatically, it simplifies the management of resources, especially in large-scale environments where manual tagging can be error-prone and time-consuming.

This configuration leverages the `mapotf` tool's capability to dynamically modify Terraform code, making it easier to enforce tagging policies across multiple resources and projects.

Before running this example, you would see [`main.tf`](./main.tf) file like this:

```hcl
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
```

You can run `mapotf transform --mptf-dir . --tf-dir .`, then you would see:

```hcl
resource "azurerm_resource_group" "this" {
  location = "West US"
  name     = "example-resources"
  tags = {
    file           = "main.tf"
    block          = "azurerm_resource_group.this"
    module_source  = try(one(data.modtm_module_source.telemetry).module_source, "")
    module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
  }

}

resource "azurerm_storage_account" "this" {
  name                     = "storageaccountname"
  resource_group_name      = azurerm_resource_group.this.name
  location                 = azurerm_resource_group.this.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags = merge({
    env = "prod"
    }, {
    file           = "main.tf"
    block          = "azurerm_storage_account.this"
    module_source  = try(one(data.modtm_module_source.telemetry).module_source, "")
    module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
  })

}

resource "azurerm_subnet" "this" {
  address_prefixes     = []
  name                 = ""
  resource_group_name  = ""
  virtual_network_name = ""
}
```

`azurerm_resource_group` and `azurerm_storage_account` supports `tags` so a default tags has been added. `azurerm_subnet` doesn't has tags, so no changes.

When `mapotf` applied default tags, the original tags on `azurerm_storage_account.this` would be honored by using `merge` function.