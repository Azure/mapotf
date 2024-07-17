data "resource" all {
}

data "provider_schema" this {
  provider_source = "hashicorp/azurerm"
  provider_version = "~> 3.0"
}

locals {
  resources_support_tags = toset([ for name, r in data.provider_schema.this.resources : name if try(r.block.attributes["tags"].type == ["map", "string"], false) ])
  resource_support_tags = flatten([ for resource_type, resource_blocks in data.resource.all.result : resource_blocks if contains(local.resources_support_tags, resource_type) ])
  mptfs = flatten([for _, blocks in local.resource_support_tags : [for b in blocks : b.mptf]])
  addresses = [for mptf in local.mptfs : mptf.block_address]
  all_resources = { for obj in flatten([for obj in flatten([for b in data.resource.all.result.* : [for nb in b : nb]]) : [for body in obj : body]]) : obj.mptf.block_address=>obj}
}

transform "update_in_place" tags {
  for_each = try(local.addresses, [])
  target_block_address = each.value
  asstring {
    tags = <<-TAGS
    %{if try(local.all_resources[each.value].tags != "", false)}merge(${local.all_resources[each.value].tags}, var.tracing_tags_enabled ? {
  file = "${local.all_resources[each.value].mptf.range.file_name}"
  block = "${local.all_resources[each.value].mptf.terraform_address}"
  module_source = try(one(data.modtm_module_source.telemetry).module_source, "")
  module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
} : {}) %{else} var.tracing_tags_enabled ? {
  file = "${local.all_resources[each.value].mptf.range.file_name}"
  block = "${local.all_resources[each.value].mptf.terraform_address}"
  module_source = try(one(data.modtm_module_source.telemetry).module_source, "")
  module_version = try(one(data.modtm_module_source.telemetry).module_version, "")
} : {}%{ endif}
TAGS
  }
}