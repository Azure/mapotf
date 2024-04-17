data "resource" "aks" {
  resource_type = "azurerm_kubernetes_cluster"
}

transform "update_in_place" "azurerm_kubernetes_cluster_ignore_changes" {
  from = data.resource.azurerm_kubernetes_cluster.aks.result
  lifecycle {
    ignore_changes = "[microsoft_defender[0].log_analytics_workspace_id, ${trimprefix(try(from.lifecycle[0].ignore_changes, "[]"), "[")}"
  }
}

data "resource" aks_node_pool {
  resource_type = "azurerm_kubernetes_cluster_node_pool"
#   use_for_each = true
}

transform "update_in_place" azurerm_kubernetes_cluster_node_pool_tags {
  from = data.resource.azurerm_kubernetes_cluster_node_pool.aks_node_pool.result
  tags = "merge(${try(from.tags, "{}")}, { key = each.key })"
}

transform "new_block" "private_endpoints_variable" {
  file         = "variables.tf"
  block_type   = "variable"
  block_labels = ["private_endpoints"]
  asraw {
    type = map(object({
      name                                    = optional(string, null)
      role_assignments                        = optional(map(object({})), {})
      lock                                    = optional(object({}), {})
      tags                                    = optional(map(any), null)
      subnet_resource_id                      = string
      private_dns_zone_group_name             = optional(string, "default")
      private_dns_zone_resource_ids           = optional(set(string), [])
      application_security_group_associations = optional(map(string), {})
      private_service_connection_name         = optional(string, null)
      network_interface_name                  = optional(string, null)
      location                                = optional(string, null)
      resource_group_name                     = optional(string, null)
      ip_configurations                       = optional(map(object({
        name               = string
        private_ip_address = string
      })), {})
    }))
    default = {}
    description = <<-DESCRIPTION
  A map of private endpoints to create on the Key Vault. The map key is deliberately arbitrary to avoid issues where map keys maybe unknown at plan time.

  - `name` - (Optional) The name of the private endpoint. One will be generated if not set.
  - `role_assignments` - (Optional) A map of role assignments to create on the private endpoint. The map key is deliberately arbitrary to avoid issues where map keys maybe unknown at plan time. See `var.role_assignments` for more information.
  - `lock` - (Optional) The lock level to apply to the private endpoint. Default is `None`. Possible values are `None`, `CanNotDelete`, and `ReadOnly`.
  - `tags` - (Optional) A mapping of tags to assign to the private endpoint.
  - `subnet_resource_id` - The resource ID of the subnet to deploy the private endpoint in.
  - `private_dns_zone_group_name` - (Optional) The name of the private DNS zone group. One will be generated if not set.
  - `private_dns_zone_resource_ids` - (Optional) A set of resource IDs of private DNS zones to associate with the private endpoint. If not set, no zone groups will be created and the private endpoint will not be associated with any private DNS zones. DNS records must be managed external to this module.
  - `application_security_group_resource_ids` - (Optional) A map of resource IDs of application security groups to associate with the private endpoint. The map key is deliberately arbitrary to avoid issues where map keys maybe unknown at plan time.
  - `private_service_connection_name` - (Optional) The name of the private service connection. One will be generated if not set.
  - `network_interface_name` - (Optional) The name of the network interface. One will be generated if not set.
  - `location` - (Optional) The Azure location where the resources will be deployed. Defaults to the location of the resource group.
  - `resource_group_name` - (Optional) The resource group where the resources will be deployed. Defaults to the resource group of the Key Vault.
  - `ip_configurations` - (Optional) A map of IP configurations to create on the private endpoint. If not specified the platform will create one. The map key is deliberately arbitrary to avoid issues where map keys maybe unknown at plan time.
    - `name` - The name of the IP configuration.
    - `private_ip_address` - The private IP address of the IP configuration.
  DESCRIPTION
    nullable    = false
  }
}

data "resource" "azurerm_cognitive_account" "cognitive_account" {
  use_for_each = false
}

transform "new_block" "private_endpoints_variable" {
  file         = "private_endpoints.tf"
  block_type   = "resource"
  block_labels = ["azurerm_private_endpoint", "this"]
  asraw {
    for_each = var.private_endpoints

    location            = "${data.resource.azurerm_cognitive_account.cognitive_account.ref}.location"
    name                = coalesce(each.value.name, "pep-${var.name}")
    resource_group_name = coalesce(each.value.resource_group_name, azurerm_cognitive_account.this.resource_group_name)
    subnet_id           = each.value.subnet_resource_id
    tags                = each.value.tags

    private_service_connection {
      is_manual_connection           = false
      name                           = coalesce(each.value.private_service_connection_name, "pse-${var.name}")
      private_connection_resource_id = "${data.resource.azurerm_cognitive_account.cognitive_account.ref}.id"
      subresource_names              = ["account"]
    }
    dynamic "ip_configuration" {
      for_each = each.value.ip_configurations

      content {
        name               = ip_configuration.value.name
        private_ip_address = ip_configuration.value.private_ip_address
        member_name        = "account"
        subresource_name   = "account"
      }
    }
    dynamic "private_dns_zone_group" {
      for_each = length(each.value.private_dns_zone_resource_ids) > 0 ? ["this"] : []

      content {
        name                 = each.value.private_dns_zone_group_name
        private_dns_zone_ids = each.value.private_dns_zone_resource_ids
      }
    }
  }
  private_service_connection {
    private_connection_resource_id = "${data.resource.azurerm_cognitive_account.cognitive_account.address}.id"
  }
}
