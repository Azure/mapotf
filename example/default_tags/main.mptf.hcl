data "resource" all {
}

data "provider_schema" this {
  provider_source = "hashicorp/azurerm"
  provider_version = "~> 3.0"
}

locals {
  resources_support_tags = toset([ for name, r in data.provider_schema.this.resources : name if try(r.block.attributes["tags"].type == ["map", "string"], false) ])
  resource_apply_tags = flatten([ for resource_type, resource_blocks in data.resource.all.result : resource_blocks if contains(local.resources_support_tags, resource_type) ])
  mptfs = flatten([for _, blocks in local.resource_apply_tags : [for b in blocks : b.mptf]])
  addresses = [for mptf in local.mptfs : mptf.block_address]
}

transform "update_in_place" tags {
  for_each = try(local.addresses, [])
  target_block_address = each.value

  asraw {
    tags = {
      hello = "world"
    }
  }
}