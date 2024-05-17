variable "prevent_destroy" {
  type    = bool
  default = false
}

data "resource" all_resource {
}

locals {
  all_resource_blocks = flatten([
    for resource_type, resource_blocks in data.resource.all_resource.result :resource_blocks
  ])
  mptfs               = flatten([for _, blocks in local.all_resource_blocks : [for b in blocks : b.mptf]])
  addresses           = [for mptf in local.mptfs : mptf.block_address]
}

transform "update_in_place" set_prevent_destroy {
  for_each             = try(local.addresses, [])
  target_block_address = each.value

  asstring {
    lifecycle {
      prevent_destroy = var.prevent_destroy
    }
  }
}