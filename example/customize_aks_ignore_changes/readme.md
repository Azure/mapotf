# Customize AKS Ignore Changes Example

This example demonstrates how to customize the `ignore_changes` lifecycle rule for an Azure Kubernetes Service (AKS) cluster using Terraform.

## Purpose

The primary purpose of the `customize_aks_ignore_changes` configuration is, sometimes you're using a public AKS module which contains an `azurerm_kubernetes_cluster` resource block, and your organization has a policy that requires you to ignore changes to certain attributes of the AKS cluster. This configuration allows you to customize the `ignore_changes` lifecycle rule for the `azurerm_kubernetes_cluster` resource block based on your organization's policy.

## Configuration

1. **Fetch All AKS Cluster Resources**: The `data "resource" aks` block is used to fetch all `azurerm_kubernetes_cluster` resources in the Terraform configuration.

```terraform
data "resource" aks {
  resource_type = "azurerm_kubernetes_cluster"
}
```

2. **Patch Resource's `lifecycle` attribute**

```terraform
transform "update_in_place" aks_ignore_changes {
  for_each = try(data.resource.aks.result.azurerm_kubernetes_cluster, {})
  target_block_address = each.value.mptf.block_address
  asstring {
    lifecycle {
      ignore_changes = "[\nmicrosoft_defender[0].log_analytics_workspace_id, ${trimprefix(try(each.value.lifecycle.0.ignore_changes, "[\n]"), "[")}"
    }
  }
}
```

In this example, the `for_each` argument is set to the result of the `azurerm_kubernetes_cluster` data source. The `target_block_address` argument is set to the block address of each resource in the collection. The `asstring` attribute is used to define a transformation that updates the `ignore_changes` lifecycle setting of each resource. The `ignore_changes` attribute is set to a list of attributes that should be ignored when determining whether to update the resource. We've tried to merge the original `ignore_changes` list with the new attribute `microsoft_defender[0].log_analytics_workspace_id`. Of course, you can define your own Mapotf `variable` block here to make `microsoft_defender[0].log_analytics_workspace_id` configurable.

## Example

```terraform
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
```

Run `terraform init`, or `mapotf init`, ensure that module `Azure/aks/azurerm` is downloaded at `.terraform/modules/aks` folder.

After running `mapotf transform -r --mptf-dir . --tf-dir .`, the `ignore_changes` argument in `lifecycle` setting would be applied to the resources based on the configuration. You can check configs under `.terraform/modules` to see the changes, all `azurerm_kubernetes_cluster` blocks declared in the referenced module would also be updated.

Or, you can run `mapotf apply -r --mptf-dir . --tf-dir .`. `mapotf` would apply the transformation and then run `terraform apply` to apply the changes to the Terraform configuration. No matter how Terraform ends up applying the changes, after apply all transformations made by `mapotf` would be reverted. These transformations only exist while Terraform is running.

Before transformation, the `ignore_chagnes` in `.terraform/modules/aks/main.tf` file would look like this:

```terraform
ignore_changes = [
  http_application_routing_enabled,
  http_proxy_config[0].no_proxy,
  kubernetes_version,
  public_network_access_enabled,
  # we might have a random suffix in cluster's name so we have to ignore it here, but we've traced user supplied cluster name by `null_resource.kubernetes_cluster_name_keeper` so when the name is changed we'll recreate this resource.
  name,
]
```

After transformation, the `ignore_chagnes` in `.terraform/modules/aks/main.tf` file would look like this:

```terraform
ignore_changes = [
  microsoft_defender[0].log_analytics_workspace_id,
  http_application_routing_enabled,
  http_proxy_config[0].no_proxy,
  kubernetes_version,
  public_network_access_enabled,
  # we might have a random suffix in cluster's name so we have to ignore it here, but we've traced user supplied cluster name by `null_resource.kubernetes_cluster_name_keeper` so when the name is changed we'll recreate this resource.
  name,
]
```

You may notice that the `microsoft_defender[0].log_analytics_workspace_id` attribute is added to the `ignore_changes` list.

In summary, this example demonstrates how to use the `mapotf` tool to control the `prevent_destroy` lifecycle setting of Terraform resources. The `update_in_place` transform block is a powerful tool that allows you to modify existing blocks in place, making it possible to add or modify lifecycle settings without recreating resources.