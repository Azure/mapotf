# Data `terraform` Block

The `data "terraform"` data block is used to retrieve `terraform` block declared in Terraform config. This data block allows you to access various attributes related to the Terraform configuration, such as required providers and required versions.

## Example Usage

```hcl
data "terraform" "example" {
}
```

## Attributes Reference

The following attributes are exported:

- **required_providers** (Optional): A map of provider configurations required by the module. Each provider configuration can include the following attributes:
    - **source** (Optional, string): The source of the provider, typically in the format `namespace/provider`.
    - **version** (Optional, string): The version constraint for the provider.

- **required_version** (Optional, string): The required version of Terraform for the module.

## Example

```hcl
data "terraform" version {

}

data "provider_schema" azurerm {
  provider_source  = data.terraform.version.required_providers["azurerm"].source
  provider_version = data.terraform.version.required_providers["azurerm"].version
}
```

In this example, the `data "terraform"` block is used to retrieve the `terraform` block declared in Terraform config, then we use retrieved `source` and `version` to get resource schemas of `azurerm` provider.

Please ensure that the provider configurations and version constraints are correctly specified to avoid runtime errors. 
